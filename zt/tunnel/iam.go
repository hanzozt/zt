package tunnel

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hanzozt/edge-api/rest_model"
	"github.com/hanzozt/edge-api/rest_util"
	apis "github.com/hanzozt/sdk-golang/edge-apis"
	"github.com/michaelquigley/pfxlog"
)

// iam logs the tunnel in by ext-jwt with a Hanzo IAM access token: the
// trimmed stdout of command, which the controller matches to the identity
// whose externalId is the token's sub. The command runs at start, a minute
// before each token's exp, and before every login after the first, because
// the SDK logs in again only once the controller has refused its session or
// its last login with a 401.
type iam struct {
	apis.BaseCredentials
	command []string

	mu     sync.Mutex
	token  string
	exp    time.Time
	logins int
}

var _ apis.Credentials = (*iam)(nil)

// newIAM fetches the first token and keeps it fresh from then on.
func newIAM(command string) (*iam, error) {
	c := &iam{command: strings.Fields(command)}
	if err := c.refresh(); err != nil {
		return nil, err
	}
	go c.keep()
	return c, nil
}

func (c *iam) Method() apis.AuthMethod { return apis.AuthMethodJwtExt }

// Payload opens every login, so a login after a refusal fetches a token first.
func (c *iam) Payload() *rest_model.Authenticate {
	c.mu.Lock()
	c.logins++
	again := c.logins > 1
	c.mu.Unlock()
	if again {
		c.must()
	}
	return c.BaseCredentials.Payload()
}

func (c *iam) AuthenticateRequest(r runtime.ClientRequest, reg strfmt.Registry) error {
	if err := c.BaseCredentials.AuthenticateRequest(r, reg); err != nil {
		return err
	}
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	return r.SetHeaderParam("Authorization", "Bearer "+token)
}

// keep refreshes a minute before exp. The ten-second floor paces a command
// that can only hand back a token already inside that minute.
func (c *iam) keep() {
	for {
		c.mu.Lock()
		wait := max(time.Until(c.exp.Add(-time.Minute)), 10*time.Second)
		c.mu.Unlock()
		time.Sleep(wait)
		c.must()
	}
}

// must refreshes or ends the tunnel: nothing else can log it in.
func (c *iam) must() {
	if err := c.refresh(); err != nil {
		pfxlog.Logger().Fatal(err)
	}
}

// refresh runs the command and reads exp without verifying the token: the
// controller verifies it, the tunnel only schedules around it.
func (c *iam) refresh() error {
	name := strings.Join(c.command, " ")
	if len(c.command) == 0 {
		return fmt.Errorf("token command is empty")
	}
	cmd := exec.Command(c.command[0], c.command[1:]...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("token command %q failed: %w", name, err)
	}
	token := strings.TrimSpace(string(out))
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return fmt.Errorf("token command %q printed no JWT: %w", name, err)
	}
	if claims.ExpiresAt == nil {
		return fmt.Errorf("token command %q printed a JWT without exp", name)
	}
	c.mu.Lock()
	c.token, c.exp = token, claims.ExpiresAt.Time
	c.mu.Unlock()
	return nil
}

// trust is the system roots, which verify the controller behind its public
// certificate, plus the network's own CAs, which verify the routers. The CAs
// are read from the controller over that verified connection.
func trust(controller string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	cas, err := rest_util.GetControllerWellKnownCasWithTlsConfig(controller, &tls.Config{RootCAs: pool})
	if err != nil {
		return nil, fmt.Errorf("reading the network CAs from %s: %w", controller, err)
	}
	for _, ca := range cas {
		pool.AddCert(ca)
	}
	return pool, nil
}
