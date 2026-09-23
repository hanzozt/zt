package tunnel

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	apis "github.com/hanzozt/sdk-golang/edge-apis"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

// controller is a fake edge client API: it answers ext-jwt logins, refusing
// the bearers in refuse with a 401, and records every bearer it was shown.
type controller struct {
	*httptest.Server
	refuse map[string]bool

	mu     sync.Mutex
	logins []string
}

func newController(t *testing.T, refuse ...string) *controller {
	c := &controller{refuse: map[string]bool{}}
	for _, r := range refuse {
		c.refuse[r] = true
	}
	c.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/edge/client/v1/version":
			_, _ = fmt.Fprint(w, `{"data":{"version":"v0","capabilities":[]},"meta":{}}`)
		case "/edge/client/v1/authenticate":
			bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			c.mu.Lock()
			c.logins = append(c.logins, r.URL.Query().Get("method")+" "+bearer)
			c.mu.Unlock()
			if c.refuse[bearer] {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":{"code":"UNAUTHORIZED","message":"refused"},"meta":{}}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":{"id":"s1","token":"session","identity":{"id":"i1","name":"sub"}},"meta":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(c.Close)
	return c
}

func (c *controller) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.logins...)
}

func (c *controller) ca() *x509.CertPool {
	return c.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
}

// login is one ext-jwt login through the SDK client the tunnel's context uses.
func (c *controller) login(creds apis.Credentials) error {
	u, _ := url.Parse(c.URL + "/edge/client/v1")
	client := apis.NewClientApiClient([]*url.URL{u}, creds.GetCaPool(), nil)
	// The client reads the controller version in the background, and a login
	// rewrites the TLS config that read dials with; let the read finish.
	client.API.ControllerSupportsOidc()
	_, err := client.Authenticate(creds, nil)
	return err
}

// tokens writes a command that prints one token per run, in order, and
// fails once they run out; runs reports how many times it ran.
func tokens(t *testing.T, ttl ...time.Duration) (command string, jwts []string, runs func() int) {
	dir := t.TempDir()
	for i, d := range ttl {
		claims := jwt.RegisteredClaims{Subject: "sub", ID: fmt.Sprint(i + 1), ExpiresAt: jwt.NewNumericDate(time.Now().Add(d))}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("k"))
		require.NoError(t, err)
		jwts = append(jwts, s)
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprint(i+1)), []byte(s+"\n"), 0o600))
	}
	script := filepath.Join(dir, "next")
	body := fmt.Sprintf("n=$(( $(cat %[1]s/n 2>/dev/null || echo 0) + 1 )); echo $n > %[1]s/n; cat %[1]s/$n\n", dir)
	require.NoError(t, os.WriteFile(script, []byte(body), 0o600))
	runs = func() int {
		b, _ := os.ReadFile(filepath.Join(dir, "n"))
		var n int
		_, _ = fmt.Sscan(string(b), &n)
		return n
	}
	return "sh " + script, jwts, runs
}

func TestLoginByExtJwt(t *testing.T) {
	ctrl := newController(t)
	command, jwts, runs := tokens(t, time.Hour)

	creds, err := newIAM(command)
	require.NoError(t, err)
	creds.CaPool = ctrl.ca()
	require.NoError(t, ctrl.login(creds))

	require.Equal(t, []string{"ext-jwt " + jwts[0]}, ctrl.seen())
	require.Equal(t, 1, runs())
}

func TestRefreshBeforeExpiry(t *testing.T) {
	ctrl := newController(t)
	// Seventy seconds out: due a minute early, so ten seconds from now.
	command, jwts, runs := tokens(t, 70*time.Second, time.Hour)

	creds, err := newIAM(command)
	require.NoError(t, err)
	creds.CaPool = ctrl.ca()
	require.Eventually(t, func() bool { return runs() == 2 }, 15*time.Second, 100*time.Millisecond)
	require.NoError(t, ctrl.login(creds))

	require.Equal(t, []string{"ext-jwt " + jwts[1]}, ctrl.seen())
}

func TestRefreshOn401(t *testing.T) {
	command, jwts, runs := tokens(t, time.Hour, time.Hour)
	ctrl := newController(t, jwts[0])

	creds, err := newIAM(command)
	require.NoError(t, err)
	creds.CaPool = ctrl.ca()
	require.Error(t, ctrl.login(creds))
	require.NoError(t, ctrl.login(creds))

	require.Equal(t, []string{"ext-jwt " + jwts[0], "ext-jwt " + jwts[1]}, ctrl.seen())
	require.Equal(t, 2, runs())
}

func TestFailingCommandIsFatal(t *testing.T) {
	_, err := newIAM("false")
	require.EqualError(t, err, `token command "false" failed: exit status 1`)

	// Once running, a command that fails ends the tunnel.
	command, jwts, _ := tokens(t, time.Hour)
	ctrl := newController(t, jwts[0])
	creds, err := newIAM(command)
	require.NoError(t, err)
	creds.CaPool = ctrl.ca()

	hook := test.NewGlobal()
	exit := logrus.StandardLogger().ExitFunc
	t.Cleanup(func() { logrus.StandardLogger().ExitFunc = exit })
	code := 0
	logrus.StandardLogger().ExitFunc = func(c int) { code = c }

	require.Error(t, ctrl.login(creds))
	_ = ctrl.login(creds)

	require.Equal(t, 1, code)
	var fatal []string
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.FatalLevel {
			fatal = append(fatal, e.Message)
		}
	}
	require.Equal(t, []string{fmt.Sprintf(`token command %q failed: exit status 1`, command)}, fatal)
}
