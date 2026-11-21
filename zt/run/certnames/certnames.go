// Package certnames keeps the controller's server certificate naming every
// address the controller is reached at.
//
// The controller's PKI is made once, at first boot, for the one address it
// advertised then. A name added later (a second, DNS-only name for clients that
// authenticate with a certificate and so cannot cross a TLS-terminating proxy)
// would otherwise never reach the certificate: TLS verification of that name
// fails, and the controller, which derives an OIDC issuer from every name its
// server certificate carries, would not answer as an issuer there either.
//
// Ensure re-issues the leaf of the server chain from the CA that issued it,
// for the same public key, carrying the names the PKI was first made with plus
// the ones asked for, so a name asked for once and later changed is dropped
// rather than kept as an alias. Nothing else about the certificate changes, the
// private key is never read, and a certificate that already names exactly those
// is left alone.
package certnames

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Ensure makes the leaf certificate of the PEM chain at chainPath name the
// names it was first issued with and every entry of names (a DNS name or an IP
// address; blanks are ignored), and no other, re-issuing it from the CA at
// signerCertPath and signerKeyPath when it does not. The first issue is the
// chain kept as chainPath+".orig" once a name has been added; before that it is
// the chain itself. The rest of the chain is kept as it was. A leaf-only copy beside the chain (the same
// name with ".cert" for ".chain.pem") is rewritten too when it holds the same
// certificate. The first rewrite keeps the original chain as chainPath+".orig".
//
// It reports whether anything was written. It refuses, writing nothing, when
// the signer did not issue the leaf: a server chain that suddenly verified
// against another CA would be a different identity, not the same one with one
// more name.
func Ensure(chainPath, signerCertPath, signerKeyPath string, names []string) (bool, error) {
	want, err := parseNames(names)
	if err != nil {
		return false, err
	}
	if len(want.dns) == 0 && len(want.ips) == 0 {
		return false, nil
	}

	chainPem, err := os.ReadFile(chainPath)
	if err != nil {
		return false, err
	}
	leafBlock, rest := pem.Decode(chainPem)
	if leafBlock == nil || leafBlock.Type != "CERTIFICATE" {
		return false, fmt.Errorf("%s: the first PEM block is not a certificate", chainPath)
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		return false, fmt.Errorf("%s: %w", chainPath, err)
	}

	first, err := firstIssue(chainPath+".orig", leaf)
	if err != nil {
		return false, err
	}
	dns := append([]string(nil), first.DNSNames...)
	ips := append([]net.IP(nil), first.IPAddresses...)
	for _, n := range want.dns {
		if !containsFold(dns, n) {
			dns = append(dns, n)
		}
	}
	for _, ip := range want.ips {
		if !containsIP(ips, ip) {
			ips = append(ips, ip)
		}
	}
	if sameNames(dns, ips, leaf) {
		return false, nil
	}

	signer, signerKey, err := loadSigner(signerCertPath, signerKeyPath)
	if err != nil {
		return false, err
	}
	if err := leaf.CheckSignatureFrom(signer); err != nil {
		return false, fmt.Errorf("%s was not issued by %s, refusing to re-issue it there: %w", chainPath, signerCertPath, err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return false, err
	}
	notBefore := time.Now().Add(-5 * time.Minute)
	if notBefore.Before(signer.NotBefore) {
		notBefore = signer.NotBefore
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               leaf.Subject,
		NotBefore:             notBefore,
		NotAfter:              leaf.NotAfter,
		KeyUsage:              leaf.KeyUsage,
		ExtKeyUsage:           leaf.ExtKeyUsage,
		UnknownExtKeyUsage:    leaf.UnknownExtKeyUsage,
		BasicConstraintsValid: leaf.BasicConstraintsValid,
		IsCA:                  leaf.IsCA,
		SubjectKeyId:          leaf.SubjectKeyId,
		DNSNames:              dns,
		IPAddresses:           ips,
		URIs:                  leaf.URIs,
		EmailAddresses:        leaf.EmailAddresses,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, leaf.PublicKey, signerKey)
	if err != nil {
		return false, fmt.Errorf("re-issuing %s: %w", chainPath, err)
	}
	newLeaf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	orig := chainPath + ".orig"
	if _, err := os.Stat(orig); errors.Is(err, os.ErrNotExist) {
		if err := writeLike(orig, chainPath, chainPem); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}

	// The leaf-only copy first: if the chain write fails after it, the next boot
	// finds the chain still short of the name and writes both again.
	if leafOnly := strings.TrimSuffix(chainPath, ".chain.pem") + ".cert"; leafOnly != chainPath {
		if old, err := os.ReadFile(leafOnly); err == nil {
			if b, _ := pem.Decode(old); b != nil && bytes.Equal(b.Bytes, leafBlock.Bytes) {
				if err := writeLike(leafOnly, leafOnly, newLeaf); err != nil {
					return false, err
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}

	out := append(append([]byte(nil), newLeaf...), bytes.TrimLeft(rest, "\r\n")...)
	if err := writeLike(chainPath, chainPath, out); err != nil {
		return false, err
	}
	return true, nil
}

// firstIssue is the certificate the PKI was first made with: the leaf of the
// chain kept at orig, when there is one for this key, else leaf itself. A kept
// chain for another key belongs to a PKI that was made again, and says nothing
// about this one.
func firstIssue(orig string, leaf *x509.Certificate) (*x509.Certificate, error) {
	pemData, err := os.ReadFile(orig)
	if errors.Is(err, os.ErrNotExist) {
		return leaf, nil
	}
	if err != nil {
		return nil, err
	}
	b, _ := pem.Decode(pemData)
	if b == nil || b.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%s: the first PEM block is not a certificate", orig)
	}
	first, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", orig, err)
	}
	if k, ok := leaf.PublicKey.(interface{ Equal(crypto.PublicKey) bool }); !ok || !k.Equal(first.PublicKey) {
		return leaf, nil
	}
	return first, nil
}

// sameNames reports whether leaf names exactly dns and ips.
func sameNames(dns []string, ips []net.IP, leaf *x509.Certificate) bool {
	if len(dns) != len(leaf.DNSNames) || len(ips) != len(leaf.IPAddresses) {
		return false
	}
	for _, n := range dns {
		if !containsFold(leaf.DNSNames, n) {
			return false
		}
	}
	for _, ip := range ips {
		if !containsIP(leaf.IPAddresses, ip) {
			return false
		}
	}
	return true
}

type wanted struct {
	dns []string
	ips []net.IP
}

func parseNames(names []string) (wanted, error) {
	var w wanted
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if ip := net.ParseIP(n); ip != nil {
			w.ips = append(w.ips, ip)
			continue
		}
		if strings.ContainsAny(n, ":/ \t") {
			return wanted{}, fmt.Errorf("%q is not a host name: give the name alone, without a scheme, port or path", n)
		}
		w.dns = append(w.dns, strings.ToLower(strings.TrimSuffix(n, ".")))
	}
	return w, nil
}

func loadSigner(certPath, keyPath string) (*x509.Certificate, crypto.Signer, error) {
	certPem, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	b, _ := pem.Decode(certPem)
	if b == nil || b.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("%s: no certificate", certPath)
	}
	cert, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", certPath, err)
	}
	keyPem, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	kb, _ := pem.Decode(keyPem)
	if kb == nil {
		return nil, nil, fmt.Errorf("%s: no PEM key", keyPath)
	}
	var key any
	switch kb.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(kb.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(kb.Bytes)
	default:
		key, err = x509.ParsePKCS8PrivateKey(kb.Bytes)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", keyPath, err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("%s: %T cannot sign", keyPath, key)
	}
	return cert, signer, nil
}

// writeLike writes data to path through a temporary file in the same
// directory, with the mode of like.
func writeLike(path, like string, data []byte) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(like); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSuffix(v, "."), s) {
			return true
		}
	}
	return false
}

func containsIP(list []net.IP, ip net.IP) bool {
	for _, v := range list {
		if v.Equal(ip) {
			return true
		}
	}
	return false
}
