package selfupdate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The fixtures are real: the checksums of a Sigstore cosign release and the keyless signature and
// certificate its own release workflow made for them.
func signedFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Sigstore signs cosign's releases as a Google service account, so the real fixture checks the other
// kind of identity (an email and the Google issuer) with the same code.
var cosignRelease = Identity{
	Subject: regexp.MustCompile(`^keyless@projectsigstore\.iam\.gserviceaccount\.com$`),
	Issuer:  "https://accounts.google.com",
}

func TestRealReleaseSignatureVerifies(t *testing.T) {
	sums, sig, cert := signedFile(t, "cosign_checksums.txt"), signedFile(t, "cosign_checksums.txt.sig"), signedFile(t, "cosign_checksums.txt.pem")
	if err := VerifySignature(sums, sig, cert, cosignRelease); err != nil {
		t.Fatalf("a real keyless signature must verify: %v", err)
	}
}

func TestRealReleaseSignatureIsRefusedWhenAnythingIsOff(t *testing.T) {
	sums, sig, cert := signedFile(t, "cosign_checksums.txt"), signedFile(t, "cosign_checksums.txt.sig"), signedFile(t, "cosign_checksums.txt.pem")
	tampered := []byte(strings.Replace(string(sums), "a", "b", 1))
	cases := map[string]error{
		"changed checksums":          VerifySignature(tampered, sig, cert, cosignRelease),
		"another project's workflow": VerifySignature(sums, sig, cert, GitHubWorkflowIdentity("sunday-studio/orion", "node-binaries.yml")),
		"another subject":            VerifySignature(sums, sig, cert, Identity{Subject: regexp.MustCompile(`^someone@example\.com$`), Issuer: cosignRelease.Issuer}),
		"another issuer":             VerifySignature(sums, sig, cert, Identity{Subject: cosignRelease.Subject, Issuer: "https://token.actions.githubusercontent.com"}),
		"garbage signature":          VerifySignature(sums, []byte("AAAA"), cert, cosignRelease),
		"no certificate":             VerifySignature(sums, sig, nil, cosignRelease),
		"garbage certificate":        VerifySignature(sums, sig, []byte("not a cert"), cosignRelease),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

// selfIssued builds a certificate chain that does not lead to Sigstore: it must be refused even when
// the identity and signature are perfect.
func TestCertificateFromAnotherAuthorityIsRefused(t *testing.T) {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "not sigstore"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	ca, _ := x509.ParseCertificate(caDER)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	uri, _ := url.Parse("https://github.com/sunday-studio/orion/.github/workflows/node-binaries.yml@refs/tags/v0.9.3")
	issuer, _ := asn1.Marshal("https://token.actions.githubusercontent.com")
	leafTmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "x"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}, URIs: []*url.URL{uri},
		ExtraExtensions: []pkix.Extension{{Id: oidIssuerV2, Value: issuer}}}
	leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	data := []byte("abc  orion-node-linux-amd64\n")
	digest := sha256.Sum256(data)
	rawSig, _ := ecdsa.SignASN1(rand.Reader, leafKey, digest[:])
	sig := []byte(base64.StdEncoding.EncodeToString(rawSig))
	id := GitHubWorkflowIdentity("sunday-studio/orion", "node-binaries.yml")

	if err := VerifySignature(data, sig, certPEM, id); err == nil || !strings.Contains(err.Error(), "not from Sigstore") {
		t.Fatalf("a certificate from another authority = %v", err)
	}
	// The same chain is accepted when that authority is trusted, which shows the refusal above is
	// about the authority and not about the rest of the check.
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
	if err := verifySignature(data, sig, certPEM, id, caPEM); err != nil {
		t.Fatalf("with its authority trusted the chain should verify: %v", err)
	}
	if err := verifySignature([]byte("changed"), sig, certPEM, id, caPEM); err == nil {
		t.Error("changed data must be refused")
	}
}
