package cli

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

var updateScreens = flag.Bool("update-screens", false, "rewrite CLI screen goldens")

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

func screenGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "screens", name+".txt")
	if *updateScreens {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run go test ./internal/cli -run Screen -update-screens): %v", path, err)
	}
	if string(want) != got {
		t.Fatalf("screen %s differs from its golden.\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func res(id, label string, st doctor.Status, msg, fix string) doctor.Result {
	return doctor.Result{ID: id, Label: label, Status: st, Message: msg, Fix: fix}
}

// artboardReport is the exact fixture drawn in docs/design/artboards/cli-doctor.html.
func artboardReport() doctor.Report {
	return doctor.Report{SchemaVersion: doctor.SchemaVersion, Checks: []doctor.Result{
		res("dns", "DNS", doctor.OK, "core.example.com resolves to 203.0.113.10", ""),
		res("tls", "TLS", doctor.OK, "valid until Jan 12, 2027", ""),
		res("core", "Core reachable", doctor.OK, "142 ms", ""),
		res("token", "Token", doctor.OK, "valid, agent edge-05", ""),
		res("clock", "Clock skew", doctor.Warn, "+4.2 s (limit 5 s)", "sudo timedatectl set-ntp true"),
		res("log-access", "Log access", doctor.Fail, "cannot read /var/log/nginx/error.log", "sudo usermod -aG adm orion && sudo systemctl restart orion-agentd"),
		res("disk", "Disk space", doctor.OK, "41 GB free", ""),
		res("service", "Service", doctor.OK, "active, running 3d 4h", ""),
		res("version", "Version", doctor.OK, "0.9.2, latest", ""),
		res("spool", "Spool", doctor.OK, "0 pending", ""),
	}}
}

// The artboard's text, character for character (colors aside). The artboard
// ends with a spinner on "Checking spool..." and counts 8 passed, so the
// finished screen is that spool check passing.
const artboardDoctorText = `
  ✓ DNS              core.example.com resolves to 203.0.113.10
  ✓ TLS              valid until Jan 12, 2027
  ✓ Core reachable   142 ms
  ✓ Token            valid, agent edge-05
  ! Clock skew       +4.2 s (limit 5 s)
      fix: sudo timedatectl set-ntp true
  ✗ Log access       cannot read /var/log/nginx/error.log
      fix: sudo usermod -aG adm orion && sudo systemctl restart orion-agentd
  ✓ Disk space       41 GB free
  ✓ Service          active, running 3d 4h
  ✓ Version          0.9.2, latest
  ✓ Spool            0 pending

  8 passed   1 warning   1 failed   exit 1
`

func TestScreenDoctorMatchesArtboardLiterally(t *testing.T) {
	for _, dark := range []bool{false, true} {
		out, exit := renderDoctor(ui.WithMode(dark), artboardReport(), 120)
		if exit != 1 {
			t.Fatalf("exit = %d, want 1 for a failed check", exit)
		}
		if got := stripANSI(out); got != artboardDoctorText {
			t.Fatalf("doctor differs from the artboard (dark=%v).\n--- artboard\n%s\n--- got\n%s", dark, artboardDoctorText, got)
		}
	}
}

func TestScreenDoctorColoursComeFromTokens(t *testing.T) {
	light, _ := renderDoctor(ui.WithMode(false), artboardReport(), 120)
	dark, _ := renderDoctor(ui.WithMode(true), artboardReport(), 120)
	if light == dark || !strings.Contains(light, "\x1b[") {
		t.Fatal("light and dark must differ and carry colour")
	}
	screenGolden(t, "doctor-light-120", light)
	screenGolden(t, "doctor-dark-120", dark)
}

func TestScreenDoctorPlainAndNarrow(t *testing.T) {
	plain, _ := renderDoctor(ui.Plain(), artboardReport(), 120)
	if strings.Contains(plain, "\x1b") || strings.Contains(plain, "✓") || !strings.Contains(plain, "[ok]") || !strings.Contains(plain, "[warn]") || !strings.Contains(plain, "[fail]") {
		t.Fatalf("plain mode must use [ok]/[warn]/[fail] and no escapes:\n%s", plain)
	}
	screenGolden(t, "doctor-plain-120", plain)

	narrow, _ := renderDoctor(ui.Plain(), artboardReport(), 80)
	for _, l := range strings.Split(narrow, "\n") {
		if len([]rune(l)) > 80 {
			t.Errorf("line wider than 80 columns (%d): %q", len([]rune(l)), l)
		}
	}
	screenGolden(t, "doctor-plain-80", narrow)
	nc, _ := renderDoctor(ui.WithMode(true), artboardReport(), 80)
	for _, l := range strings.Split(stripANSI(nc), "\n") {
		if len([]rune(l)) > 80 {
			t.Errorf("coloured line wider than 80 columns (%d): %q", len([]rune(l)), l)
		}
	}
}

func TestScreenDoctorNoConfigAndAllOK(t *testing.T) {
	noCfg := doctor.Report{Checks: []doctor.Result{
		res("config", "Config", doctor.Fail, "config file /etc/orion/config.yaml not found; run `orion-agent setup`, or set ORION_CORE_URL to run without a file", "orion-agent setup   (or fix the file: orion-agent config edit)"),
		res("dns", "DNS", doctor.Skip, "no core_url yet", ""),
		res("service", "Service", doctor.Fail, "not installed", "orion-agent service install   (prints the one-line installer)"),
	}}
	for _, w := range []int{80, 120} {
		out, exit := renderDoctor(ui.Plain(), noCfg, w)
		if exit != 1 {
			t.Fatalf("exit = %d", exit)
		}
		for _, l := range strings.Split(out, "\n") {
			if len(l) > w {
				t.Errorf("width %d: line is %d wide: %q", w, len(l), l)
			}
		}
		screenGolden(t, "doctor-noconfig-"+strings.TrimSpace(strings.Repeat(" ", 0))+itoa(w), out)
	}
	ok := doctor.Report{Checks: []doctor.Result{res("dns", "DNS", doctor.OK, "x", ""), res("tls", "TLS", doctor.OK, "y", "")}}
	out, exit := renderDoctor(ui.Plain(), ok, 80)
	if exit != 0 || !strings.Contains(out, "2 passed   0 warnings   0 failed   exit 0") {
		t.Fatalf("all-ok summary wrong (exit %d):\n%s", exit, out)
	}
	screenGolden(t, "doctor-allok-80", out)
}

func itoa(n int) string {
	if n == 80 {
		return "80"
	}
	return "120"
}
