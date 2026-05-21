package config

import (
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Image.Name != DefaultImageName {
		t.Errorf("unexpected image name: %s", cfg.Image.Name)
	}
	if cfg.Image.StrictInitImage != DefaultStrictInitImage {
		t.Errorf("unexpected strict init image: %s", cfg.Image.StrictInitImage)
	}
	if cfg.Network.Mode != "practical" {
		t.Errorf("unexpected network mode: %s", cfg.Network.Mode)
	}
	if cfg.Network.Backend != "proxy" {
		t.Errorf("unexpected network backend: %s", cfg.Network.Backend)
	}
	if cfg.Network.DefaultAction != "allow" {
		t.Errorf("unexpected network defaultAction: %s", cfg.Network.DefaultAction)
	}
	if cfg.Resources.CPUs != 4 {
		t.Errorf("unexpected cpus: %d", cfg.Resources.CPUs)
	}
	if cfg.Resources.Memory != "4g" {
		t.Errorf("unexpected memory: %s", cfg.Resources.Memory)
	}
	if cfg.Network.ProxyPort != 18080 {
		t.Errorf("unexpected proxy port: %d", cfg.Network.ProxyPort)
	}
	if cfg.Audit.Commands.Enabled {
		t.Error("expected command audit to be disabled by default")
	}
	if cfg.Audit.Commands.Backend != "ebpf" {
		t.Errorf("unexpected command audit backend: %s", cfg.Audit.Commands.Backend)
	}
	if cfg.Audit.Commands.FailClosed {
		t.Error("expected command audit failClosed to default false")
	}
	if cfg.Audit.Commands.LogArgs != "full" {
		t.Errorf("unexpected command audit logArgs: %s", cfg.Audit.Commands.LogArgs)
	}
	if cfg.Audit.Commands.MaxArgs != 64 {
		t.Errorf("unexpected command audit maxArgs: %d", cfg.Audit.Commands.MaxArgs)
	}
	if cfg.Audit.Commands.MaxArgBytes != 16384 {
		t.Errorf("unexpected command audit maxArgBytes: %d", cfg.Audit.Commands.MaxArgBytes)
	}
}

func TestValidate(t *testing.T) {
	cfg := Defaults()
	if err := Validate(cfg); err != nil {
		t.Errorf("expected defaults to be valid: %v", err)
	}
	cfg.Network.Mode = "invalid"
	if err := Validate(cfg); err == nil {
		t.Error("expected invalid mode to fail validation")
	}
}

func TestValidateBackend(t *testing.T) {
	cfg := Defaults()
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	if err := Validate(cfg); err != nil {
		t.Errorf("expected ebpf backend with initImage to be valid: %v", err)
	}

	cfg.Network.Backend = "invalid"
	if err := Validate(cfg); err == nil {
		t.Error("expected invalid backend to fail validation")
	}
}

func TestValidateBackendRequiresInitImage(t *testing.T) {
	cfg := Defaults()
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = ""
	if err := Validate(cfg); err == nil {
		t.Error("expected ebpf backend without initImage to fail validation")
	}
}

func TestValidateCommandAudit(t *testing.T) {
	cfg := Defaults()
	cfg.Audit.Commands.Enabled = true

	cfg.Audit.Commands.Backend = "proxy"
	if err := Validate(cfg); err == nil {
		t.Error("expected unsupported command audit backend to fail")
	}

	cfg = Defaults()
	cfg.Audit.Commands.Enabled = true
	cfg.Audit.Commands.LogArgs = "redacted"
	if err := Validate(cfg); err == nil {
		t.Error("expected unsupported command audit logArgs to fail")
	}

	cfg = Defaults()
	cfg.Audit.Commands.Enabled = true
	cfg.Audit.Commands.MaxArgs = -1
	if err := Validate(cfg); err == nil {
		t.Error("expected negative command audit maxArgs to fail")
	}
}

func TestValidateDefaultAction(t *testing.T) {
	cfg := Defaults()
	cfg.Network.DefaultAction = "deny"
	if err := Validate(cfg); err != nil {
		t.Errorf("expected deny defaultAction to be valid: %v", err)
	}
	cfg.Network.DefaultAction = "invalid"
	if err := Validate(cfg); err == nil {
		t.Error("expected invalid defaultAction to fail validation")
	}
}

func TestValidateOffRequiresDeny(t *testing.T) {
	cfg := Defaults()
	cfg.Network.Mode = "off"
	cfg.Network.DefaultAction = "allow"
	if err := Validate(cfg); err == nil {
		t.Error("expected mode off with defaultAction allow to fail validation")
	}
	cfg.Network.DefaultAction = "deny"
	if err := Validate(cfg); err == nil {
		t.Error("expected mode off with proxy backend to fail validation")
	}
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	if err := Validate(cfg); err != nil {
		t.Errorf("expected mode off with ebpf backend and defaultAction deny to be valid: %v", err)
	}
}

func TestValidateVersion(t *testing.T) {
	cfg := Defaults()
	cfg.Version = 2
	if err := Validate(cfg); err == nil {
		t.Error("expected version 2 to fail validation")
	}
}

func TestValidateMemory(t *testing.T) {
	tests := []struct {
		mem  string
		want bool
	}{
		{"4g", true},
		{"512m", true},
		{"1G", true},
		{"2tb", true},
		{"", true},
		{"invalid", false},
		{"4 g", false},
	}
	for _, tt := range tests {
		t.Run(tt.mem, func(t *testing.T) {
			err := ValidateMemory(tt.mem)
			if tt.want && err != nil {
				t.Errorf("expected %q to be valid, got %v", tt.mem, err)
			}
			if !tt.want && err == nil {
				t.Errorf("expected %q to be invalid", tt.mem)
			}
		})
	}
}

func TestValidateEmptyLists(t *testing.T) {
	cfg := Defaults()
	cfg.Network.Blocklist = []string{""}
	if err := Validate(cfg); err == nil {
		t.Error("expected empty blocklist entry to fail")
	}
	cfg = Defaults()
	cfg.Network.Allowlist = []string{"  "}
	if err := Validate(cfg); err == nil {
		t.Error("expected empty allowlist entry to fail")
	}
}
