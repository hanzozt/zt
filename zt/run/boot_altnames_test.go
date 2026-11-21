package run

import (
	"bytes"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/hanzozt/identity"
	"github.com/hanzozt/zt/v2/zt/constants"
)

// firstBoot lays out the PKI an empty ZT_HOME gets, for the address advertised
// then, the way Controller does: pkiEnv, then CreateMinimalPki.
func firstBoot(t *testing.T, advertised string) {
	t.Helper()
	// pkiEnv writes these with os.Setenv; registering them first restores them.
	for _, v := range []string{constants.HomeVarName, constants.PkiCtrlCAVarName, constants.PkiCtrlKeyVarName,
		constants.PkiCtrlServerCertVarName, constants.PkiCtrlCertVarName, constants.PkiSignerCertVarName,
		constants.PkiSignerKeyVarName, constants.CtrlEdgeAltAdvertisedAddressVarName} {
		t.Setenv(v, "")
	}
	t.Setenv(constants.CtrlAdvertisedAddressVarName, advertised)
	pki := &QuickstartOpts{Home: t.TempDir(), TrustDomain: "zt", InstanceID: "ctrl", out: io.Discard, errOut: io.Discard}
	pki.pkiEnv()
	pki.CreateMinimalPki()
}

// loaded reads the controller's identity the way controller/config does.
func loaded(t *testing.T) identity.Identity {
	t.Helper()
	id, err := identity.LoadIdentity(identity.Config{
		Key:        os.Getenv(constants.PkiCtrlKeyVarName),
		Cert:       os.Getenv(constants.PkiCtrlCertVarName),
		ServerCert: os.Getenv(constants.PkiCtrlServerCertVarName),
		CA:         os.Getenv(constants.PkiCtrlCAVarName),
	})
	if err != nil {
		t.Fatalf("the controller could not load its identity: %v", err)
	}
	return id
}

// handshake serves TLS with the controller's identity and dials it as an
// enrolled client does: trusting only the network's root, asking for name,
// presenting a client certificate.
func handshake(t *testing.T, id identity.Identity, name string) error {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", id.ServerTLSConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_ = c.(*tls.Conn).Handshake()
		_ = c.Close()
	}()

	rootPem, err := os.ReadFile(os.Getenv(constants.PkiCtrlCAVarName))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPem) {
		t.Fatal("no root in " + os.Getenv(constants.PkiCtrlCAVarName))
	}
	cfg := id.ClientTLSConfig().Clone()
	cfg.ServerName, cfg.RootCAs = name, roots
	c, err := tls.Dial("tcp", ln.Addr().String(), cfg)
	if err == nil {
		_ = c.Close()
	}
	return err
}

func TestAnAlternativeNameReachesTheCertificateTheControllerServes(t *testing.T) {
	firstBoot(t, "zt-api.example.test")

	before := loaded(t)
	if err := handshake(t, before, "zt-api.example.test"); err != nil {
		t.Fatalf("first boot does not serve its own address: %v", err)
	}
	if err := handshake(t, before, "zt-ctrl.example.test"); err == nil {
		t.Fatal("first boot already verifies for the alternative name, so this test proves nothing")
	}

	// Unset, boot leaves the certificate alone.
	chain := os.Getenv(constants.PkiCtrlServerCertVarName)
	first, err := os.ReadFile(chain)
	if err != nil {
		t.Fatal(err)
	}
	if err := nameAlternatives(); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(chain); !bytes.Equal(now, first) {
		t.Fatal("no alternative name configured, and the certificate changed")
	}

	t.Setenv(constants.CtrlEdgeAltAdvertisedAddressVarName, "zt-ctrl.example.test")
	if err := nameAlternatives(); err != nil {
		t.Fatal(err)
	}

	after := loaded(t)
	for _, name := range []string{"zt-ctrl.example.test", "zt-api.example.test", "localhost", "127.0.0.1"} {
		if err := handshake(t, after, name); err != nil {
			t.Errorf("does not verify for %s after the re-issue: %v", name, err)
		}
	}

	// The OIDC issuers are derived from these chains (controller/webapis/oidc-api.go).
	chains := after.GetX509ActiveServerCertChains()
	if len(chains) != 1 || !slices.Contains(chains[0][0].DNSNames, "zt-ctrl.example.test") {
		t.Fatalf("the served chain does not name the alternative: %v", chains)
	}
	if !chains[0][0].PublicKey.(interface{ Equal(crypto.PublicKey) bool }).Equal(before.GetX509ActiveServerCertChains()[0][0].PublicKey) {
		t.Fatal("the re-issued certificate is for another key")
	}

	// Every later boot finds it named and writes nothing.
	named, _ := os.ReadFile(chain)
	if err := nameAlternatives(); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(chain); !bytes.Equal(now, named) {
		t.Fatal("a second boot re-issued a certificate that already named everything")
	}
}
