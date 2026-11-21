package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanzozt/zt/v2/controller/config"
	"github.com/hanzozt/zt/v2/zt/constants"
	"gopkg.in/yaml.v3"
)

// bindings reads the APIs each web listener of a controller config serves.
func bindings(t *testing.T, cfg string) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Web []struct {
			Name string `yaml:"name"`
			Apis []struct {
				Binding string `yaml:"binding"`
			} `yaml:"apis"`
		} `yaml:"web"`
	}
	if err := yaml.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, w := range c.Web {
		for _, a := range w.Apis {
			out[w.Name] = append(out[w.Name], a.Binding)
		}
	}
	return out
}

// On the config the controller writes at first boot, the client listener is
// added beside the all-APIs one, serves the sign-in APIs alone, and the
// controller still loads the file.
func TestTheClientListenerJoinsTheConfigTheControllerWrote(t *testing.T) {
	firstBoot(t, "zt-api.hanzo.ai")
	cfg := filepath.Join(t.TempDir(), "ctrl.yaml")
	if err := writeControllerConfig(cfg, "zt-api.hanzo.ai", "443"); err != nil {
		t.Fatal(err)
	}

	t.Setenv(constants.CtrlEdgeClientPortVarName, "")
	if err := clientListener(cfg); err != nil {
		t.Fatal(err)
	}
	if got := bindings(t, cfg); len(got) != 1 {
		t.Fatalf("an unset port still added a listener: %v", got)
	}

	t.Setenv(constants.CtrlEdgeClientPortVarName, "1281")
	t.Setenv(constants.CtrlEdgeAltAdvertisedAddressVarName, "")
	if err := clientListener(cfg); err == nil {
		t.Fatal("a port with no name to advertise was accepted")
	}

	t.Setenv(constants.CtrlEdgeAltAdvertisedAddressVarName, "zt-ctrl.hanzo.ai")
	if err := clientListener(cfg); err != nil {
		t.Fatal(err)
	}
	got := bindings(t, cfg)
	if len(got) != 2 {
		t.Fatalf("listeners: %v", got)
	}
	if client := strings.Join(got["client"], ","); client != "edge-client,edge-oidc" {
		t.Errorf("the client listener serves %s, want edge-client,edge-oidc", client)
	}
	for _, b := range got["client"] {
		if b == "edge-management" || b == "fabric" {
			t.Errorf("the client listener serves %s", b)
		}
	}
	if len(got["client-management"]) < 3 {
		t.Errorf("the first listener lost its APIs: %v", got["client-management"])
	}

	// The controller makes its database directory at first boot.
	if err := os.MkdirAll(filepath.Join(os.Getenv(constants.HomeVarName), "db"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadConfig(cfg); err != nil {
		t.Errorf("the controller cannot load the converged config: %v", err)
	}
}
