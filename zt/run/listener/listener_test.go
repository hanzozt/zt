package listener

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// first is the shape the controller's first boot writes: one listener that
// binds every API, with its own identity and options.
const first = `v: 3
db: /state/db/ctrl.db
web:
  - name: client-management
    bindPoints:
      - interface: 0.0.0.0:1280
        address: zt-api.hanzo.ai:443
    identity:
      ca: /state/pki/root/certs/root.cert
      key: /state/pki/ctrl-intermediate/keys/ctrl-server.key
      server_cert: /state/pki/ctrl-intermediate/certs/ctrl-server.chain.pem
      cert: /state/pki/ctrl-intermediate/certs/ctrl-client.cert
    options:
      idleTimeout: 5000ms
      minTLSVersion: TLS1.2
    apis:
      - binding: edge-management
        options: { }
      - binding: edge-client
        options: { }
      - binding: fabric
        options: { }
      - binding: edge-oidc
        options: { }
`

func config(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ctrl.yaml")
	if err := os.WriteFile(p, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

type api struct {
	Binding string `yaml:"binding"`
}

type listenerConfig struct {
	Name       string `yaml:"name"`
	BindPoints []struct {
		Interface string `yaml:"interface"`
		Address   string `yaml:"address"`
	} `yaml:"bindPoints"`
	Identity map[string]string `yaml:"identity"`
	Options  map[string]any    `yaml:"options"`
	Apis     []api             `yaml:"apis"`
}

func read(t *testing.T, p string) (map[string]any, []listenerConfig) {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var all struct {
		Web []listenerConfig `yaml:"web"`
	}
	if err := yaml.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	var rest map[string]any
	if err := yaml.Unmarshal(raw, &rest); err != nil {
		t.Fatal(err)
	}
	return rest, all.Web
}

func bindings(l listenerConfig) []string {
	var b []string
	for _, a := range l.Apis {
		b = append(b, a.Binding)
	}
	return b
}

// The new listener serves sign-in and nothing of management, binds beside the
// first, and is advertised at its own name on the first's port.
func TestTheListenerServesWhatAnIdentitySignsInWithAndNothingElse(t *testing.T) {
	p := config(t)
	changed, err := Ensure(p, "1281", "ctrl.zt.hanzo.ai")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	_, web := read(t, p)
	if len(web) != 2 {
		t.Fatalf("want 2 listeners, got %d", len(web))
	}

	all, narrow := web[0], web[1]
	if got := strings.Join(bindings(all), ","); got != "edge-management,edge-client,fabric,edge-oidc" {
		t.Errorf("the first listener changed: %s", got)
	}
	if narrow.Name != Name {
		t.Errorf("name %q", narrow.Name)
	}
	if got := strings.Join(bindings(narrow), ","); got != "edge-client,edge-oidc" {
		t.Errorf("the second listener serves %s, want edge-client,edge-oidc", got)
	}
	bp := narrow.BindPoints
	if len(bp) != 1 || bp[0].Interface != "0.0.0.0:1281" || bp[0].Address != "ctrl.zt.hanzo.ai:443" {
		t.Errorf("bindPoints %+v", bp)
	}
	for k, v := range all.Identity {
		if narrow.Identity[k] != v {
			t.Errorf("identity %s: %q, want the first listener's %q", k, narrow.Identity[k], v)
		}
	}
	if narrow.Options["idleTimeout"] != all.Options["idleTimeout"] {
		t.Errorf("options not taken from the first listener: %v", narrow.Options)
	}
}

// Everything outside the web section is as it was.
func TestTheRestOfTheConfigIsKept(t *testing.T) {
	p := config(t)
	if _, err := Ensure(p, "1281", "ctrl.zt.hanzo.ai"); err != nil {
		t.Fatal(err)
	}
	rest, _ := read(t, p)
	if rest["db"] != "/state/db/ctrl.db" || rest["v"] != 3 {
		t.Errorf("config changed: %v", rest)
	}
}

// A second boot with the same values writes nothing; a changed value rewrites.
func TestEveryBootConverges(t *testing.T) {
	p := config(t)
	if _, err := Ensure(p, "1281", "ctrl.zt.hanzo.ai"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	changed, err := Ensure(p, "1281", "ctrl.zt.hanzo.ai")
	if err != nil || changed {
		t.Fatalf("a second boot wrote: changed=%v err=%v", changed, err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Error("a second boot rewrote the file")
	}

	changed, err = Ensure(p, "1290", "ctrl.zt.hanzo.ai")
	if err != nil || !changed {
		t.Fatalf("a new port was not written: changed=%v err=%v", changed, err)
	}
	_, web := read(t, p)
	if len(web) != 2 || web[1].BindPoints[0].Interface != "0.0.0.0:1290" {
		t.Errorf("the listener was added again or not moved: %+v", web)
	}
}

// The first rewrite keeps the original beside it, and only the first.
func TestTheOriginalIsKept(t *testing.T) {
	p := config(t)
	if _, err := Ensure(p, "1281", "ctrl.zt.hanzo.ai"); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(p, "1290", "ctrl.zt.hanzo.ai"); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(p + ".orig")
	if err != nil || string(orig) != first {
		t.Errorf("the original was not kept as it was: %v", err)
	}
}

func TestWhatItCannotUseIsRefused(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) string {
		p := filepath.Join(dir, "c.yaml")
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, c := range map[string]string{
		"no web section":    "v: 3\n",
		"web is not a list": "web: {}\n",
		"no bind points":    "web:\n  - name: a\n    apis: []\n",
		"no listener":       "web: []\n",
	} {
		if _, err := Ensure(write(c), "1281", "h"); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := Ensure(config(t), "", "h"); err == nil {
		t.Error("no port: no error")
	}
	if _, err := Ensure(config(t), "1281", ""); err == nil {
		t.Error("no host: no error")
	}
	if _, err := Ensure(filepath.Join(dir, "missing.yaml"), "1281", "h"); err == nil {
		t.Error("no file: no error")
	}
}
