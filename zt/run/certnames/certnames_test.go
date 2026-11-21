package certnames

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pki lays out what the controller's first boot leaves behind: a root, an
// intermediate that signs the server, the server's leaf on its own and as a
// chain with the intermediate.
type pki struct {
	dir                    string
	root, inter            *x509.Certificate
	interKey               crypto.Signer
	chain, leafOnly        string
	interCert, interKeyPem string
	leafKey                crypto.Signer
}

func issue(t *testing.T, tmpl, parent *x509.Certificate, pub crypto.PublicKey, key crypto.Signer) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func certPem(c *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}

func newPki(t *testing.T, rsaIntermediate bool) *pki {
	t.Helper()
	p := &pki{dir: t.TempDir()}
	now := time.Now()

	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "root-ca"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(10 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	p.root = issue(t, rootT, rootT, rootKey.Public(), rootKey)

	var keyPem []byte
	if rsaIntermediate {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		p.interKey = k
		keyPem = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	} else {
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		p.interKey = k
		der, _ := x509.MarshalECPrivateKey(k)
		keyPem = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	}
	interT := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "intermediate-ca-ctrl"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(5 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, MaxPathLen: 1, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	p.inter = issue(t, interT, p.root, p.interKey.Public(), rootKey)

	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	p.leafKey = leafKey
	spiffe, _ := url.Parse("spiffe://zt/controller/ctrl")
	leafT := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "ctrl"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{"localhost", "zt-api.hanzo.ai"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		URIs:        []*url.URL{spiffe}}
	leaf := issue(t, leafT, p.inter, leafKey.Public(), p.interKey)

	certs := filepath.Join(p.dir, "intermediate-ca-ctrl", "certs")
	keys := filepath.Join(p.dir, "intermediate-ca-ctrl", "keys")
	for _, d := range []string{certs, keys} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	p.chain = filepath.Join(certs, "server.chain.pem")
	p.leafOnly = filepath.Join(certs, "server.cert")
	p.interCert = filepath.Join(certs, "intermediate-ca-ctrl.cert")
	p.interKeyPem = filepath.Join(keys, "intermediate-ca-ctrl.key")
	write(t, p.chain, append(certPem(leaf), certPem(p.inter)...), 0o644)
	write(t, p.leafOnly, certPem(leaf), 0o644)
	write(t, p.interCert, certPem(p.inter), 0o644)
	write(t, p.interKeyPem, keyPem, 0o600)
	return p
}

func write(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// blocks returns every certificate of a PEM file, in order.
func blocks(t *testing.T, data []byte) []*x509.Certificate {
	t.Helper()
	var out []*x509.Certificate
	for {
		var b *pem.Block
		b, data = pem.Decode(data)
		if b == nil {
			return out
		}
		c, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
}

func (p *pki) roots() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(p.root)
	return pool
}

func (p *pki) inters() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(p.inter)
	return pool
}

func (p *pki) verifies(t *testing.T, leaf *x509.Certificate, name string) {
	t.Helper()
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: name, Roots: p.roots(), Intermediates: p.inters()}); err != nil {
		t.Fatalf("leaf does not verify for %s: %v", name, err)
	}
}

func TestAnAddedNameIsOnTheSameIdentity(t *testing.T) {
	for _, rsaInter := range []bool{false, true} {
		p := newPki(t, rsaInter)
		before := read(t, p.chain)
		old := blocks(t, before)[0]

		changed, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{" zt-ctrl.hanzo.ai ", ""})
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("a missing name was not added")
		}

		got := blocks(t, read(t, p.chain))
		if len(got) != 2 || !got[1].Equal(p.inter) {
			t.Fatalf("the chain lost its intermediate: %d certificates", len(got))
		}
		leaf := got[0]
		if want := []string{"localhost", "zt-api.hanzo.ai", "zt-ctrl.hanzo.ai"}; len(leaf.DNSNames) != 3 ||
			leaf.DNSNames[0] != want[0] || leaf.DNSNames[1] != want[1] || leaf.DNSNames[2] != want[2] {
			t.Fatalf("DNS names %v, want %v", leaf.DNSNames, want)
		}
		if !leaf.PublicKey.(*ecdsa.PublicKey).Equal(p.leafKey.Public()) {
			t.Fatal("the re-issued certificate is for another key")
		}
		if len(leaf.IPAddresses) != 2 || len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://zt/controller/ctrl" {
			t.Fatalf("lost a SAN: ips %v uris %v", leaf.IPAddresses, leaf.URIs)
		}
		if leaf.Subject.CommonName != old.Subject.CommonName || !leaf.NotAfter.Equal(old.NotAfter) ||
			leaf.KeyUsage != old.KeyUsage || len(leaf.ExtKeyUsage) != 2 {
			t.Fatal("something other than the names changed")
		}
		if leaf.SerialNumber.Cmp(old.SerialNumber) == 0 {
			t.Fatal("re-issued under the old serial")
		}
		p.verifies(t, leaf, "zt-ctrl.hanzo.ai")
		p.verifies(t, leaf, "zt-api.hanzo.ai")

		if only := blocks(t, read(t, p.leafOnly)); len(only) != 1 || !only[0].Equal(leaf) {
			t.Fatal("the leaf-only copy was not rewritten with the chain")
		}
		if !bytes.Equal(read(t, p.chain+".orig"), before) {
			t.Fatal("the original chain was not kept")
		}
		if fi, _ := os.Stat(p.chain); fi.Mode().Perm() != 0o644 {
			t.Fatalf("mode %v, want the original 0644", fi.Mode().Perm())
		}
	}
}

func TestANamedCertificateIsLeftAlone(t *testing.T) {
	p := newPki(t, false)
	if _, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{"zt-ctrl.hanzo.ai"}); err != nil {
		t.Fatal(err)
	}
	once := read(t, p.chain)

	for _, names := range [][]string{{"zt-ctrl.hanzo.ai"}, {"ZT-CTRL.hanzo.ai."}, {}, {" "}} {
		changed, err := Ensure(p.chain, p.interCert, p.interKeyPem, names)
		if err != nil {
			t.Fatal(err)
		}
		if changed || !bytes.Equal(read(t, p.chain), once) {
			t.Fatalf("%q rewrote a certificate that already named it", names)
		}
	}
}

// A name that was asked for and no longer is does not stay as an alias: the
// certificate carries the names it was first made with and the ones asked for
// now.
func TestARenamedNameReplacesTheOldOne(t *testing.T) {
	p := newPki(t, false)
	if _, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{"old.hanzo.ai"}); err != nil {
		t.Fatal(err)
	}
	changed, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{"zt-ctrl.hanzo.ai"})
	if err != nil || !changed {
		t.Fatalf("changed %v, err %v", changed, err)
	}
	leaf := blocks(t, read(t, p.chain))[0]
	if want := []string{"localhost", "zt-api.hanzo.ai", "zt-ctrl.hanzo.ai"}; len(leaf.DNSNames) != 3 ||
		leaf.DNSNames[0] != want[0] || leaf.DNSNames[1] != want[1] || leaf.DNSNames[2] != want[2] {
		t.Fatalf("DNS names %v, want %v", leaf.DNSNames, want)
	}
	p.verifies(t, leaf, "zt-ctrl.hanzo.ai")
	p.verifies(t, leaf, "zt-api.hanzo.ai")
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "old.hanzo.ai", Roots: p.roots(), Intermediates: p.inters()}); err == nil {
		t.Fatal("the old name still verifies")
	}
	if len(leaf.IPAddresses) != 2 || len(leaf.URIs) != 1 {
		t.Fatalf("lost a first-issue SAN: ips %v uris %v", leaf.IPAddresses, leaf.URIs)
	}

	again, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{"zt-ctrl.hanzo.ai"})
	if err != nil || again {
		t.Fatalf("a second boot wrote: changed %v, err %v", again, err)
	}
}

func TestNothingAskedReadsNothing(t *testing.T) {
	if changed, err := Ensure("/nonexistent/chain.pem", "/nonexistent/ca.cert", "/nonexistent/ca.key", []string{"", "  "}); err != nil || changed {
		t.Fatalf("changed %v, err %v", changed, err)
	}
}

func TestAnIPBecomesAnIPName(t *testing.T) {
	p := newPki(t, false)
	if _, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{"203.0.113.7"}); err != nil {
		t.Fatal(err)
	}
	leaf := blocks(t, read(t, p.chain))[0]
	if len(leaf.DNSNames) != 2 || len(leaf.IPAddresses) != 3 || !leaf.IPAddresses[2].Equal(net.ParseIP("203.0.113.7")) {
		t.Fatalf("dns %v ips %v", leaf.DNSNames, leaf.IPAddresses)
	}
}

func TestAnAddressIsNotAName(t *testing.T) {
	p := newPki(t, false)
	before := read(t, p.chain)
	for _, n := range []string{"zt-ctrl.hanzo.ai:443", "https://zt-ctrl.hanzo.ai", "ctrl zt"} {
		if _, err := Ensure(p.chain, p.interCert, p.interKeyPem, []string{n}); err == nil {
			t.Fatalf("%q was accepted as a host name", n)
		}
	}
	if !bytes.Equal(read(t, p.chain), before) {
		t.Fatal("a refused name still wrote the chain")
	}
}

func TestAnotherCAMayNotReissue(t *testing.T) {
	p, other := newPki(t, false), newPki(t, false)
	before := read(t, p.chain)
	if _, err := Ensure(p.chain, other.interCert, other.interKeyPem, []string{"zt-ctrl.hanzo.ai"}); err == nil {
		t.Fatal("a CA that did not issue the leaf re-issued it")
	}
	if !bytes.Equal(read(t, p.chain), before) {
		t.Fatal("the refusal still wrote the chain")
	}
	if _, err := os.Stat(p.chain + ".orig"); err == nil {
		t.Fatal("the refusal left a backup behind")
	}
}
