package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func TestGeneratePolicyFile(t *testing.T) {
	dir := t.TempDir()
	if err := GeneratePolicyFile(dir, "practical", "proxy", "allow", []string{"example.com"}, []string{"safe.example.com"}); err != nil {
		t.Fatalf("GeneratePolicyFile failed: %v", err)
	}

	path := filepath.Join(dir, "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected policy file: %v", err)
	}

	var policy PolicyFile
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatalf("parsing policy file: %v", err)
	}

	if policy.Mode != "practical" {
		t.Errorf("expected mode practical, got %s", policy.Mode)
	}
	if policy.Backend != "proxy" {
		t.Errorf("expected backend proxy, got %s", policy.Backend)
	}
	if policy.DefaultAction != "allow" {
		t.Errorf("expected defaultAction allow, got %s", policy.DefaultAction)
	}
	if len(policy.Blocklist) != 1 || policy.Blocklist[0] != "example.com" {
		t.Errorf("unexpected blocklist: %v", policy.Blocklist)
	}
	if len(policy.Allowlist) != 1 || policy.Allowlist[0] != "safe.example.com" {
		t.Errorf("unexpected allowlist: %v", policy.Allowlist)
	}
}

func TestGeneratePolicyBundle(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "strict",
			Backend:       "ebpf",
			DefaultAction: "allow",
			FailClosed:    true,
			Blocklist:     []string{"*.segment.io"},
			Allowlist:     []string{"api.openai.com"},
			EBPF: config.EffectiveEBPF{
				MirrorProjectEvents: true,
			},
		},
		Audit: config.EffectiveAudit{
			Rotation: config.EffectiveAuditRotation{
				MaxBytes: 10485760,
				MaxFiles: 5,
			},
			Commands: config.EffectiveCommandAudit{
				Enabled:             true,
				Backend:             "ebpf",
				FailClosed:          false,
				LogArgs:             "full",
				MaxArgs:             64,
				MaxArgBytes:         16384,
				IncludeExecutables:  []string{"/usr/bin/curl"},
				ExcludeExecutables:  []string{"/usr/bin/true"},
				IncludeCwd:          []string{"/workspace"},
				ExcludeCwd:          []string{"/tmp"},
				MirrorProjectEvents: true,
			},
		},
	}

	if err := GeneratePolicyBundle(dir, "opencode-sandbox-abc123", "/projects/myapp", "myapp", cfg); err != nil {
		t.Fatalf("GeneratePolicyBundle failed: %v", err)
	}

	path := filepath.Join(dir, "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected policy bundle file: %v", err)
	}

	var bundle PolicyBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("parsing policy bundle: %v", err)
	}

	if bundle.Version != 1 {
		t.Errorf("expected version 1, got %d", bundle.Version)
	}
	if bundle.RunID != "opencode-sandbox-abc123" {
		t.Errorf("unexpected runId: %s", bundle.RunID)
	}
	if bundle.Project.Path != "/projects/myapp" {
		t.Errorf("unexpected project path: %s", bundle.Project.Path)
	}
	if bundle.Project.Name != "myapp" {
		t.Errorf("unexpected project name: %s", bundle.Project.Name)
	}
	if bundle.Network.Mode != "strict" {
		t.Errorf("unexpected network mode: %s", bundle.Network.Mode)
	}
	if bundle.Network.Backend != "ebpf" {
		t.Errorf("unexpected backend: %s", bundle.Network.Backend)
	}
	if bundle.Network.DefaultAction != "allow" {
		t.Errorf("unexpected defaultAction: %s", bundle.Network.DefaultAction)
	}
	if !bundle.Network.FailClosed {
		t.Error("expected failClosed true")
	}
	if len(bundle.Rules.BlockDomains) != 1 || bundle.Rules.BlockDomains[0] != "*.segment.io" {
		t.Errorf("unexpected blockDomains: %v", bundle.Rules.BlockDomains)
	}
	if len(bundle.Rules.AllowDomains) != 1 || bundle.Rules.AllowDomains[0] != "api.openai.com" {
		t.Errorf("unexpected allowDomains: %v", bundle.Rules.AllowDomains)
	}
	if bundle.Resolver.TTLMinSeconds != 30 {
		t.Errorf("unexpected ttlMinSeconds: %d", bundle.Resolver.TTLMinSeconds)
	}
	if bundle.Resolver.TTLMaxSeconds != 300 {
		t.Errorf("unexpected ttlMaxSeconds: %d", bundle.Resolver.TTLMaxSeconds)
	}
	if bundle.Audit.Events.HostJsonl != "/sandbox/logs/audit-events.jsonl" {
		t.Errorf("unexpected audit hostJsonl: %s", bundle.Audit.Events.HostJsonl)
	}
	if bundle.Audit.Events.ProjectMirrorJsonl != "/workspace/.opencode-sandbox/audit-events.jsonl" {
		t.Errorf("unexpected audit projectMirrorJsonl: %s", bundle.Audit.Events.ProjectMirrorJsonl)
	}
	if !bundle.Audit.Events.MirrorProjectEvents {
		t.Error("expected audit mirrorProjectEvents true")
	}
	if bundle.Audit.Events.Rotation.MaxBytes == 0 || bundle.Audit.Events.Rotation.MaxFiles == 0 {
		t.Errorf("expected audit rotation defaults: %+v", bundle.Audit.Events.Rotation)
	}
	if bundle.Events.HostJsonl != "/sandbox/logs/audit-events.jsonl" {
		t.Errorf("unexpected hostJsonl: %s", bundle.Events.HostJsonl)
	}
	if bundle.Events.ProjectMirrorJsonl != "/workspace/.opencode-sandbox/audit-events.jsonl" {
		t.Errorf("unexpected projectMirrorJsonl: %s", bundle.Events.ProjectMirrorJsonl)
	}
	if !bundle.Events.MirrorProjectEvents {
		t.Error("expected mirrorProjectEvents true")
	}
	if !bundle.Audit.Commands.Enabled {
		t.Error("expected command audit enabled")
	}
	if bundle.Audit.Commands.Backend != "ebpf" {
		t.Errorf("unexpected command audit backend: %s", bundle.Audit.Commands.Backend)
	}
	if bundle.Audit.Commands.FailClosed {
		t.Error("expected command audit failClosed false")
	}
	if bundle.Audit.Commands.LogArgs != "full" {
		t.Errorf("unexpected command audit logArgs: %s", bundle.Audit.Commands.LogArgs)
	}
	if bundle.Audit.Commands.HostJsonl != "/sandbox/logs/audit-events.jsonl" {
		t.Errorf("unexpected command hostJsonl: %s", bundle.Audit.Commands.HostJsonl)
	}
	if bundle.Audit.Commands.ProjectMirrorJsonl != "/workspace/.opencode-sandbox/audit-events.jsonl" {
		t.Errorf("unexpected command projectMirrorJsonl: %s", bundle.Audit.Commands.ProjectMirrorJsonl)
	}
	if len(bundle.Audit.Commands.IncludeExecutables) != 1 || bundle.Audit.Commands.IncludeExecutables[0] != "/usr/bin/curl" {
		t.Errorf("unexpected command includeExecutables: %v", bundle.Audit.Commands.IncludeExecutables)
	}

	// Verify backward-compatibility fields are present for the proxy.
	if bundle.Mode != "strict" {
		t.Errorf("unexpected top-level mode: %s", bundle.Mode)
	}
	if len(bundle.Blocklist) != 1 || bundle.Blocklist[0] != "*.segment.io" {
		t.Errorf("unexpected top-level blocklist: %v", bundle.Blocklist)
	}
	if len(bundle.Allowlist) != 1 || bundle.Allowlist[0] != "api.openai.com" {
		t.Errorf("unexpected top-level allowlist: %v", bundle.Allowlist)
	}
}

func TestGeneratePolicyBundleNoSecretsOrURLs(t *testing.T) {
	dir := t.TempDir()
	cfg := config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:       "practical",
			Backend:    "proxy",
			Blocklist:  []string{"example.com"},
			Allowlist:  []string{"safe.example.com"},
			FailClosed: true,
		},
	}

	if err := GeneratePolicyBundle(dir, "run-1", "/projects/test", "test", cfg); err != nil {
		t.Fatalf("GeneratePolicyBundle failed: %v", err)
	}

	path := filepath.Join(dir, "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading bundle: %v", err)
	}

	content := string(data)
	forbidden := []string{"http://", "https://", "token", "secret", "password", "api_key"}
	for _, f := range forbidden {
		if containsIgnoreCase(content, f) {
			t.Errorf("bundle should not contain %q", f)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}
