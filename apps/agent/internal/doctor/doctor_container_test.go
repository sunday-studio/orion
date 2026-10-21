package doctor

import (
	"context"
	"testing"
)

func TestServiceCheckIsSkippedInsideAContainer(t *testing.T) {
	t.Setenv("ORION_CONTAINER", "1")
	e := &Env{Service: func() (bool, bool, string) { return false, false, "" }}
	if r := checkService(context.Background(), e); r.Status != Skip {
		t.Fatalf("a container has no service manager to install into: %+v", r)
	}
}
