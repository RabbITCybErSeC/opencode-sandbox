package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	sandboxruntime "github.com/RabbITCybErSeC/opencode-sandbox/internal/runtime"
)

func TestRunWithDefaults(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()
	checks := Run(cfg)
	if len(checks) == 0 {
		t.Fatal("expected checks")
	}

	for _, c := range checks {
		if c.ID == "ebpf.init-image" && c.Status != StatusSkip {
			t.Errorf("expected ebpf.init-image to be skipped by default, got %s", c.Status)
		}
		if c.ID == "ebpf.support" && c.Status != StatusSkip {
			t.Errorf("expected ebpf.support to be skipped with proxy backend, got %s", c.Status)
		}
	}
}

func TestRunWithEBPFMissingInitImage(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = ""

	checks := Run(cfg)
	var found bool
	for _, c := range checks {
		if c.ID == "ebpf.init-image" {
			found = true
			if c.Status != StatusFail {
				t.Errorf("expected fail for missing init image, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected ebpf.init-image check")
	}
}

func TestRunWithDefaultCommandAuditMissingInitImageWarns(t *testing.T) {
	withImageInspect(t, errors.New("image not found"))
	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()
	cfg.Network.Mode = "practical"
	cfg.Network.Backend = "proxy"
	cfg.Audit.Commands.Enabled = true

	checks := Run(cfg)
	var found bool
	for _, c := range checks {
		if c.ID == "ebpf.init-image" {
			found = true
			if c.Status != StatusWarn {
				t.Fatalf("expected warn for missing command audit init image, got %s", c.Status)
			}
			if !strings.Contains(c.Message, "disable command audit") {
				t.Fatalf("expected command audit guidance, got %q", c.Message)
			}
		}
	}
	if !found {
		t.Error("expected ebpf.init-image check")
	}
}

func TestRunReportsMissingAuditLog(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()

	checks := Run(cfg)
	check := findCheck(t, checks, "audit.logs")
	if check.Status != StatusWarn {
		t.Fatalf("expected warn for missing audit log, got %+v", check)
	}
	if !strings.Contains(check.Message, audit.DefaultFileName) {
		t.Fatalf("expected audit log filename in message, got %q", check.Message)
	}
}

func TestRunReportsLatestAuditLog(t *testing.T) {
	withImageInspect(t, nil)
	base := t.TempDir()
	runDir := filepath.Join(base, "run-1")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, audit.DefaultFileName), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Audit.EventLog = base

	checks := Run(cfg)
	check := findCheck(t, checks, "audit.logs")
	if check.Status != StatusPass {
		t.Fatalf("expected pass for existing audit log, got %+v", check)
	}
}

func TestRunReportsOpenCodeStateWarningWithoutMutation(t *testing.T) {
	withImageInspect(t, nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	paths, err := sandboxruntime.ResolveOpenCodeStatePaths()
	if err != nil {
		t.Fatal(err)
	}
	dataDir := paths.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(dataDir, "auth.json")
	if err := os.WriteFile(authPath, []byte("   "), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()
	checks := Run(cfg)
	check := findCheck(t, checks, "opencode.state")
	if check.Status != StatusWarn {
		t.Fatalf("expected warn for malformed auth, got %+v", check)
	}
	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "   " {
		t.Fatalf("doctor should not mutate auth.json, got %q", data)
	}
}

func TestRunSkipsOpenCodeStateWhenHostDataDisabled(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
	cfg.OpenCode.MountHostData = false
	cfg.Audit.EventLog = t.TempDir()

	checks := Run(cfg)
	check := findCheck(t, checks, "opencode.state")
	if check.Status != StatusSkip {
		t.Fatalf("expected skip when host data disabled, got %+v", check)
	}
}

func TestRunWithEBPFMissingNetworkName(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
	cfg.Audit.EventLog = t.TempDir()
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	cfg.Network.EBPF.NetworkName = ""

	checks := Run(cfg)
	var found bool
	for _, c := range checks {
		if c.ID == "ebpf.network-name" {
			found = true
			if c.Status != StatusWarn {
				t.Errorf("expected warn for missing network name, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected ebpf.network-name check")
	}
}

func TestImageInspectArgsUsesAppleImageSubcommand(t *testing.T) {
	args := imageInspectArgs("opencode-sandbox-init:latest")
	want := []string{"image", "inspect", "opencode-sandbox-init:latest"}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestIsHealthyAllPass(t *testing.T) {
	checks := []Check{
		{ID: "a", Status: StatusPass},
		{ID: "b", Status: StatusPass},
	}
	if !IsHealthy(checks) {
		t.Error("expected healthy")
	}
}

func TestIsHealthyWithFail(t *testing.T) {
	checks := []Check{
		{ID: "a", Status: StatusPass},
		{ID: "b", Status: StatusFail},
	}
	if IsHealthy(checks) {
		t.Error("expected unhealthy")
	}
}

func TestIsHealthyWithWarnOnly(t *testing.T) {
	checks := []Check{
		{ID: "a", Status: StatusWarn},
		{ID: "b", Status: StatusWarn},
	}
	if !IsHealthy(checks) {
		t.Error("expected healthy with only warnings")
	}
}

func withImageInspect(t *testing.T, err error) {
	t.Helper()
	oldInspect := inspectDoctorImage
	inspectDoctorImage = func(image string) ([]byte, error) {
		if err != nil {
			return []byte("Image not found"), err
		}
		return nil, nil
	}
	t.Cleanup(func() { inspectDoctorImage = oldInspect })
}

func findCheck(t *testing.T, checks []Check, id string) Check {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("expected check %s", id)
	return Check{}
}
