// Command gen-roots refreshes fulcio-roots.pem from Sigstore's public trusted root.
//
//	go run ./internal/selfupdate/gen-roots -out internal/selfupdate/fulcio-roots.pem
//
// Run it when Sigstore rotates its certificate authority; until then signature checks on releases
// fail closed.
package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const source = "https://raw.githubusercontent.com/sigstore/root-signing/main/targets/trusted_root.json"

type trustedRoot struct {
	CertificateAuthorities []struct {
		CertChain struct {
			Certificates []struct {
				RawBytes string `json:"rawBytes"`
			} `json:"certificates"`
		} `json:"certChain"`
	} `json:"certificateAuthorities"`
}

func main() {
	out := flag.String("out", "internal/selfupdate/fulcio-roots.pem", "file to write")
	flag.Parse()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(source)
	if err != nil {
		fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail(fmt.Errorf("%s answered %d", source, resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		fail(err)
	}
	var root trustedRoot
	if err := json.Unmarshal(body, &root); err != nil {
		fail(err)
	}
	var b strings.Builder
	b.WriteString("# Sigstore public-good Fulcio certificate authorities, taken from\n# https://github.com/sigstore/root-signing (targets/trusted_root.json). Root and intermediate\n# certificates only: they are public. Refresh with: go run ./internal/selfupdate/gen-roots\n\n")
	count := 0
	for _, ca := range root.CertificateAuthorities {
		for _, c := range ca.CertChain.Certificates {
			der, err := base64.StdEncoding.DecodeString(c.RawBytes)
			if err != nil {
				fail(err)
			}
			b.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
			count++
		}
	}
	if count == 0 {
		fail(fmt.Errorf("no certificates found in %s", source))
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %d certificates to %s\n", count, *out)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-roots:", err)
	os.Exit(1)
}
