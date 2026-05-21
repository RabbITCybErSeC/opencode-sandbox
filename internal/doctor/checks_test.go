package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func TestRunWithDefaults(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
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

func TestRunWithEBPFMissingNetworkName(t *testing.T) {
	withImageInspect(t, nil)
	cfg := config.Defaults()
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
