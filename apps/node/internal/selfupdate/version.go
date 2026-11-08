package selfupdate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v?(\d{1,4})\.(\d{1,4})\.(\d{1,4})(?:-([0-9A-Za-z.]{1,32}))?$`)

type semver struct {
	nums [3]int
	pre  string
}

func parseVersion(v string) (semver, bool) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return semver{}, false
	}
	var out semver
	for i := 0; i < 3; i++ {
		out.nums[i], _ = strconv.Atoi(m[i+1])
	}
	out.pre = m[4]
	return out, true
}

// compare orders two versions: negative when a is older. ok is false when either is not a release
// version, such as a "dev" build.
func compare(a, b string) (cmp int, ok bool) {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range pa.nums {
		if pa.nums[i] != pb.nums[i] {
			if pa.nums[i] < pb.nums[i] {
				return -1, true
			}
			return 1, true
		}
	}
	switch {
	case pa.pre == pb.pre:
		return 0, true
	case pa.pre == "":
		return 1, true
	case pb.pre == "":
		return -1, true
	case pa.pre < pb.pre:
		return -1, true
	}
	return 1, true
}

// Tag is the release tag for a version: "v" plus the version.
func Tag(version string) string { return "v" + strings.TrimPrefix(strings.TrimSpace(version), "v") }

// AssetName is the release file for an operating system and architecture.
func AssetName(goos, goarch string) (string, error) {
	switch goos {
	case "linux", "darwin":
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}
	return "orion-node-" + goos + "-" + goarch, nil
}
