package doctor

import (
	"context"
	"os"
	"strings"
	"testing"

	"orion/node/internal/config"
)

func TestServiceCheckIsSkippedInsideAContainer(t *testing.T) {
	t.Setenv("ORION_CONTAINER", "1")
	e := &Env{Service: func() (bool, bool, string) { return false, false, "" }}
	if r := checkService(context.Background(), e); r.Status != Skip {
		t.Fatalf("a container has no service manager to install into: %+v", r)
	}
}

func TestLogHintsNameTheRestartThatWorksOnThisPlatform(t *testing.T) {
	missing := func(string) error { return os.ErrNotExist }
	for goos, want := range map[string]string{"linux": "sudo systemctl restart orion-node", "darwin": "orion-node restart"} {
		r := checkLogAccess(context.Background(), &Env{GOOS: goos, Config: &config.UserConfig{Logging: config.LoggingConfig{Path: "/x/node.log"}}, CanRead: missing})
		if !strings.Contains(r.Fix, want) || (goos == "darwin" && strings.Contains(r.Fix, "systemctl")) {
			t.Errorf("%s fix = %q, want it to say %q", goos, r.Fix, want)
		}
	}
}
