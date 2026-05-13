package collector

import (
	"errors"
	"testing"
)

func TestRunPM2MonitorWithRunner(t *testing.T) {
	t.Run("online app is up", func(t *testing.T) {
		result := runPM2MonitorWithRunner(
			PM2MonitorConfig{AppName: "worker"},
			func(name string, args ...string) ([]byte, error) {
				assertPM2JlistCommand(t, name, args)
				return []byte(`[{"name":"worker","pid":1234,"monit":{"memory":1048576,"cpu":1.5},"pm2_env":{"status":"online","restart_time":2,"pm_uptime":1767225600000}}]`), nil
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
		if result.Metrics.PM2Status != "online" {
			t.Fatalf("pm2_status = %q, want online", result.Metrics.PM2Status)
		}
		if result.Metrics.PID != 1234 {
			t.Fatalf("pid = %d, want 1234", result.Metrics.PID)
		}
	})

	t.Run("stopped app is down", func(t *testing.T) {
		result := runPM2MonitorWithRunner(
			PM2MonitorConfig{AppName: "worker"},
			func(name string, args ...string) ([]byte, error) {
				assertPM2JlistCommand(t, name, args)
				return []byte(`[{"name":"worker","pid":0,"monit":{"memory":0,"cpu":0},"pm2_env":{"status":"stopped","restart_time":3}}]`), nil
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Metrics.FailureReason != "pm2 app is not online" {
			t.Fatalf("failure_reason = %q, want pm2 app is not online", result.Metrics.FailureReason)
		}
	})

	t.Run("missing app is down", func(t *testing.T) {
		result := runPM2MonitorWithRunner(
			PM2MonitorConfig{AppName: "worker"},
			func(name string, args ...string) ([]byte, error) {
				return []byte(`[{"name":"api","pm2_env":{"status":"online"}}]`), nil
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "pm2 app not found" {
			t.Fatalf("error = %+v, want pm2 app not found", result.Error)
		}
	})

	t.Run("pm2 failure is down", func(t *testing.T) {
		result := runPM2MonitorWithRunner(
			PM2MonitorConfig{AppName: "worker"},
			func(name string, args ...string) ([]byte, error) {
				return nil, errors.New("pm2 unavailable")
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "pm2 unavailable" {
			t.Fatalf("error = %+v, want pm2 unavailable", result.Error)
		}
	})
}

func assertPM2JlistCommand(t *testing.T, name string, args []string) {
	t.Helper()

	if name != "pm2" {
		t.Fatalf("command = %q, want pm2", name)
	}
	if len(args) != 1 || args[0] != "jlist" {
		t.Fatalf("args = %#v, want [jlist]", args)
	}
}
