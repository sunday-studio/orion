//go:build ignore

// mkrelease makes a signed fake release for the self-update end-to-end test: it signs SHA256SUMS the way
// Sigstore keyless signing does (a short-lived certificate naming the release workflow, a base64
// signature, base64 PEM) but with a private test authority.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	dir := flag.String("dir", "release", "release root")
	caDir := flag.String("ca", "ca", "folder for the test authority")
	version := flag.String("version", "", "release version and tag")
	binary := flag.String("binary", "", "the node binary of this release")
	asset := flag.String("asset", "", "asset name")
	flag.Parse()
	must(os.MkdirAll(*caDir, 0o755))
	caKeyPath, caPath := filepath.Join(*caDir, "ca.key"), filepath.Join(*caDir, "roots.pem")
	var caKey *ecdsa.PrivateKey
	var caCert *x509.Certificate
	if data, err := os.ReadFile(caKeyPath); err == nil {
		block, _ := pem.Decode(data)
		caKey, err = x509.ParseECPrivateKey(block.Bytes)
		must(err)
		pemBytes, _ := os.ReadFile(caPath)
		b, _ := pem.Decode(pemBytes)
		caCert, err = x509.ParseCertificate(b.Bytes)
		must(err)
	} else {
		caKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "orion e2e authority"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(240 * time.Hour),
			IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
		must(err)
		caCert, _ = x509.ParseCertificate(der)
		keyDER, _ := x509.MarshalECPrivateKey(caKey)
		must(os.WriteFile(caKeyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
		must(os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
	}

	rel := filepath.Join(*dir, *version)
	must(os.MkdirAll(rel, 0o755))
	in, err := os.Open(*binary)
	must(err)
	out, err := os.OpenFile(filepath.Join(rel, *asset), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	must(err)
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), in)
	must(err)
	must(out.Close())
	sums := hex.EncodeToString(h.Sum(nil)) + "  " + *asset + "\n"
	must(os.WriteFile(filepath.Join(rel, "SHA256SUMS"), []byte(sums), 0o644))

	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	uri, _ := url.Parse("https://github.com/sunday-studio/orion/.github/workflows/node-binaries.yml@refs/heads/main")
	issuer, _ := asn1.Marshal("https://token.actions.githubusercontent.com")
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "release"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(10 * time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}, URIs: []*url.URL{uri},
		ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}, Value: issuer}}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafKey.PublicKey, caKey)
	must(err)
	digest := sha256.Sum256([]byte(sums))
	sig, err := ecdsa.SignASN1(rand.Reader, leafKey, digest[:])
	must(err)
	must(os.WriteFile(filepath.Join(rel, "SHA256SUMS.sig"), []byte(base64.StdEncoding.EncodeToString(sig)), 0o644))
	must(os.WriteFile(filepath.Join(rel, "SHA256SUMS.pem"), []byte(base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))), 0o644))
	fmt.Println(hex.EncodeToString(h.Sum(nil)))
}
