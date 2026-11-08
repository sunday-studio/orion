//go:build orion_e2e

package cli

import (
	node "orion/node/internal"
	"os"
	"strings"

	"orion/node/internal/selfupdate"
)

// Compiled only into the binaries of the end-to-end update test (`-tags orion_e2e`). It lets the
// test serve releases from a local mirror, sign them with its own authority and make a new version
// crash after the start check. A release build never contains this file.
func init() {
	if base := os.Getenv("ORION_E2E_RELEASE_BASE"); base != "" {
		releaseBase = func(repo, tag string) string { return strings.TrimRight(base, "/") + "/" + tag }
	}
	if rootsFile := os.Getenv("ORION_E2E_ROOTS"); rootsFile != "" {
		verifyRelease = func(sums, sig, cert []byte) error {
			roots, err := os.ReadFile(rootsFile)
			if err != nil {
				return err
			}
			return selfupdate.VerifySignatureWithRoots(sums, sig, cert, selfupdate.GitHubWorkflowIdentity(releaseRepo, releaseWorkflow, releaseRef), roots)
		}
	}
	if version := os.Getenv("ORION_E2E_CRASH_VERSION"); version != "" && version == node.Version {
		afterUpdateBegin = func() { os.Exit(1) }
	}
}
