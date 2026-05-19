package cli

import (
	"runtime"
	"testing"
)

func TestReleaseAssetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "latest",
			version: "latest",
			want:    "https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-darwin-arm64",
		},
		{
			name:    "pinned version",
			version: "v0.1.1",
			want:    "https://github.com/sunday-studio/orion/releases/download/v0.1.1/orion-agent-darwin-arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := releaseAssetURL("sunday-studio/orion", tt.version, "orion-agent-darwin-arm64")
			if got != tt.want {
				t.Fatalf("releaseAssetURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReleaseAssetName(t *testing.T) {
	t.Parallel()

	got, err := releaseAssetName()
	if err != nil {
		t.Fatalf("releaseAssetName() error = %v", err)
	}

	want := "orion-agent-" + runtime.GOOS + "-" + runtime.GOARCH
	if got != want {
		t.Fatalf("releaseAssetName() = %q, want %q", got, want)
	}
}
