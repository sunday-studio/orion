package selfupdate

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Releases are signed by their GitHub Actions workflow with Sigstore keyless signing: a short-lived
// certificate from Fulcio names the workflow, and the key it certifies signs SHA256SUMS. Checking
// that signature means a changed release file alone is not enough to get a binary installed; the
// attacker would have to run the release workflow.
//
// This verifies the certificate chain, the workflow identity, the OIDC issuer and the signature, all
// offline. It does not consult the transparency log. The certificate is only valid for minutes, so
// it is checked as of the moment it was issued. A replayed old release is still refused by the
// "newer than the running version" rule.

//go:embed fulcio-roots.pem
var fulcioRootsPEM []byte

// Identity is who must have signed.
type Identity struct {
	// Subject must match a URI or email in the certificate, for example the release workflow of the repository.
	Subject *regexp.Regexp
	// Issuer is the OIDC issuer the certificate was issued for.
	Issuer string
}

// GitHubWorkflowIdentity is the identity of a repository's workflow file run on one ref. The ref is
// pinned: anyone who can start the workflow on some other branch can get a valid signature from
// Sigstore, so a certificate for another ref is not accepted.
func GitHubWorkflowIdentity(repo, workflow, ref string) Identity {
	return Identity{
		Subject: regexp.MustCompile("^https://github\\.com/" + regexp.QuoteMeta(repo) + "/\\.github/workflows/" + regexp.QuoteMeta(workflow) + "@" + regexp.QuoteMeta(ref) + "$"),
		Issuer:  "https://token.actions.githubusercontent.com",
	}
}

var (
	oidIssuerV1 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	oidIssuerV2 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
)

// VerifySignature checks that data was signed by id. sig and cert are the contents of the release's
// SHA256SUMS.sig and SHA256SUMS.pem (both base64, the certificate wrapping PEM).
func VerifySignature(data, sig, cert []byte, id Identity) error {
	return verifySignature(data, sig, cert, id, fulcioRootsPEM)
}

// VerifySignatureWithRoots is VerifySignature with other trusted certificate authorities. It exists
// for tests that sign with their own authority; the Node itself always uses the embedded Sigstore roots.
func VerifySignatureWithRoots(data, sig, cert []byte, id Identity, rootsPEM []byte) error {
	return verifySignature(data, sig, cert, id, rootsPEM)
}

func verifySignature(data, sig, cert []byte, id Identity, rootsPEM []byte) error {
	certs, err := parseCertificates(cert)
	if err != nil || len(certs) == 0 {
		return errors.New("the release certificate cannot be read")
	}
	leaf := certs[0]
	roots, intermediates := x509.NewCertPool(), x509.NewCertPool()
	for _, c := range mustCertificates(rootsPEM) {
		if c.CheckSignatureFrom(c) == nil && c.Subject.String() == c.Issuer.String() {
			roots.AddCert(c)
		} else {
			intermediates.AddCert(c)
		}
	}
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots: roots, Intermediates: intermediates,
		CurrentTime: leaf.NotBefore.Add(time.Second), // the certificate lives for minutes
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}); err != nil {
		return fmt.Errorf("the release certificate is not from Sigstore: %w", err)
	}
	if issuer := certIssuer(leaf); issuer != id.Issuer {
		return fmt.Errorf("the release was signed for issuer %q, not %q", issuer, id.Issuer)
	}
	subjects := make([]string, 0, len(leaf.URIs)+len(leaf.EmailAddresses))
	for _, u := range leaf.URIs {
		subjects = append(subjects, u.String())
	}
	subjects = append(subjects, leaf.EmailAddresses...)
	matched := false
	for _, subject := range subjects {
		if id.Subject.MatchString(subject) {
			matched = true
		}
	}
	if !matched {
		return errors.New("the release was not signed by this project's release workflow")
	}
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("the release certificate does not hold an ECDSA key")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil {
		return errors.New("the release signature cannot be read")
	}
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(pub, digest[:], raw) {
		return errors.New("the signature does not match the release checksums")
	}
	return nil
}

// parseCertificates accepts the certificate as PEM or as base64 of PEM, which is how cosign writes it.
func parseCertificates(in []byte) ([]*x509.Certificate, error) {
	text := strings.TrimSpace(string(in))
	if !strings.Contains(text, "-----BEGIN") {
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return nil, err
		}
		text = string(decoded)
	}
	var out []*x509.Certificate
	rest := []byte(text)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out, nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
}

func mustCertificates(pemBytes []byte) []*x509.Certificate {
	certs, err := parseCertificates(pemBytes)
	if err != nil {
		panic("selfupdate: bad embedded certificates: " + err.Error())
	}
	return certs
}

// certIssuer reads the OIDC issuer Fulcio put in the certificate.
func certIssuer(c *x509.Certificate) string {
	for _, ext := range c.Extensions {
		switch {
		case ext.Id.Equal(oidIssuerV2):
			var s string
			if _, err := asn1.Unmarshal(ext.Value, &s); err == nil {
				return s
			}
		case ext.Id.Equal(oidIssuerV1):
			return string(ext.Value)
		}
	}
	return ""
}

var _ pkix.Extension
