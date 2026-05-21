package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, ".opencode-sandbox.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	found, err := FindProjectConfig(child)
	if err != nil {
		t.Fatalf("FindProjectConfig failed: %v", err)
	}
	if found != configPath {
		t.Errorf("expected %s, got %s", configPath, found)
	}
}

func TestGlobalConfigPath(t *testing.T) {
	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath failed: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 1
network:
  mode: strict
  blocklist:
    - example.com
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Version == nil || *cfg.Version != 1 {
		t.Error("expected version 1")
	}
	if cfg.Network == nil || cfg.Network.Mode == nil || *cfg.Network.Mode != "strict" {
		t.Error("expected network mode strict")
	}
	if cfg.Network.Blocklist == nil || len(*cfg.Network.Blocklist) != 1 {
		t.Error("expected one blocklist entry")
	}
}

func TestLoadUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 1
unknownKey: value
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestLoadUnknownNestedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 1
network:
  mode: practical
  unknownNested: value
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown nested key")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMergeDefaultsOnly(t *testing.T) {
	base := Defaults()
	merged := MergeEffective(base)
	if merged.Image.Name != DefaultImageName {
		t.Error("unexpected image name")
	}
}

func TestMergeOverrideScalar(t *testing.T) {
	base := Defaults()
	overlay := File{
		Image: &Image{Name: ptr("custom:latest")},
	}
	merged := MergeEffective(base, overlay)
	if merged.Image.Name != "custom:latest" {
		t.Errorf("expected custom:latest, got %s", merged.Image.Name)
	}
}

func TestMergeNilPreservesDefault(t *testing.T) {
	base := Defaults()
	overlay := File{}
	merged := MergeEffective(base, overlay)
	if merged.Image.Name != DefaultImageName {
		t.Error("expected default image name to be preserved")
	}
}

func TestMergeNetworkInheritGlobalTrue(t *testing.T) {
	base := Defaults()
	base.Network.Blocklist = []string{"global.example.com"}

	overlay := File{
		Network: &Network{
			InheritGlobal: ptr(true),
			Blocklist:     &[]string{"project.example.com"},
		},
	}

	merged := MergeEffective(base, overlay)
	if len(merged.Network.Blocklist) != 2 {
		t.Fatalf("expected 2 blocklist entries, got %d: %v", len(merged.Network.Blocklist), merged.Network.Blocklist)
	}
	if merged.Network.Blocklist[0] != "global.example.com" {
		t.Errorf("expected first entry to be global, got %s", merged.Network.Blocklist[0])
	}
	if merged.Network.Blocklist[1] != "project.example.com" {
		t.Errorf("expected second entry to be project, got %s", merged.Network.Blocklist[1])
	}
}

func TestMergeNetworkInheritGlobalFalse(t *testing.T) {
	base := Defaults()
	base.Network.Blocklist = []string{"global.example.com"}

	overlay := File{
		Network: &Network{
			InheritGlobal: ptr(false),
			Blocklist:     &[]string{"project.example.com"},
		},
	}

	merged := MergeEffective(base, overlay)
	if len(merged.Network.Blocklist) != 1 {
		t.Fatalf("expected 1 blocklist entry, got %d", len(merged.Network.Blocklist))
	}
	if merged.Network.Blocklist[0] != "project.example.com" {
		t.Errorf("expected project entry, got %s", merged.Network.Blocklist[0])
	}
}

func TestMergeNetworkInheritGlobalDefault(t *testing.T) {
	// When inheritGlobal is not explicitly set, it defaults to true.
	base := Defaults()
	base.Network.Blocklist = []string{"global.example.com"}

	overlay := File{
		Network: &Network{
			Blocklist: &[]string{"project.example.com"},
		},
	}

	merged := MergeEffective(base, overlay)
	if len(merged.Network.Blocklist) != 2 {
		t.Fatalf("expected 2 entries (default inheritGlobal=true), got %d", len(merged.Network.Blocklist))
	}
}

func TestMergeNetworkAllowlist(t *testing.T) {
	base := Defaults()
	base.Network.Allowlist = []string{"global-ok.example.com"}

	overlay := File{
		Network: &Network{
			InheritGlobal: ptr(true),
			Allowlist:     &[]string{"project-ok.example.com"},
		},
	}

	merged := MergeEffective(base, overlay)
	if len(merged.Network.Allowlist) != 2 {
		t.Fatalf("expected 2 allowlist entries, got %d", len(merged.Network.Allowlist))
	}
}

func TestMergeMultipleOverlays(t *testing.T) {
	base := Defaults()
	global := File{
		Network: &Network{
			Blocklist: &[]string{"global.example.com"},
		},
	}
	project := File{
		Network: &Network{
			InheritGlobal: ptr(true),
			Blocklist:     &[]string{"project.example.com"},
		},
	}

	merged := MergeEffective(base, global, project)
	if len(merged.Network.Blocklist) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(merged.Network.Blocklist))
	}
}

func TestMergeProjectOverridesGlobalMode(t *testing.T) {
	base := Defaults()
	global := File{
		Network: &Network{
			Mode: ptr("strict"),
		},
	}
	project := File{
		Network: &Network{
			Mode: ptr("off"),
		},
	}

	merged := MergeEffective(base, global, project)
	if merged.Network.Mode != "off" {
		t.Errorf("expected mode off, got %s", merged.Network.Mode)
	}
}

func TestResolvePaths(t *testing.T) {
	cfg := Defaults()
	cfg.Skills.ImportedDir = "~/.config/opencode-sandbox/skills"
	ResolvePaths(&cfg, "/tmp")
	if cfg.Skills.ImportedDir == "~/.config/opencode-sandbox/skills" {
		t.Error("expected tilde to be expanded")
	}
}

func TestLoadEBPFConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  ebpf:
    initImage: opencode-sandbox-init:latest
    networkName: opencode-sandbox
    eventLog: ~/.local/state/opencode-sandbox/runs
    mirrorProjectEvents: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Network == nil || cfg.Network.Backend == nil || *cfg.Network.Backend != "ebpf" {
		t.Error("expected network backend ebpf")
	}
	if cfg.Network.DefaultAction == nil || *cfg.Network.DefaultAction != "allow" {
		t.Error("expected network defaultAction allow")
	}
	if cfg.Network.EBPF == nil {
		t.Fatal("expected ebpf config")
	}
	if cfg.Network.EBPF.InitImage == nil || *cfg.Network.EBPF.InitImage != "opencode-sandbox-init:latest" {
		t.Error("expected ebpf initImage")
	}
	if cfg.Network.EBPF.NetworkName == nil || *cfg.Network.EBPF.NetworkName != "opencode-sandbox" {
		t.Error("expected ebpf networkName")
	}
	if cfg.Network.EBPF.MirrorProjectEvents == nil || !*cfg.Network.EBPF.MirrorProjectEvents {
		t.Error("expected ebpf mirrorProjectEvents true")
	}
}

func TestLoadCommandAuditConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 1
audit:
  commands:
    enabled: true
    backend: ebpf
    failClosed: false
    logArgs: full
    maxArgs: 32
    maxArgBytes: 8192
    includeExecutables:
      - /usr/bin/curl
    excludeExecutables:
      - /usr/bin/true
    includeCwd:
      - /workspace
    excludeCwd:
      - /tmp
    mirrorProjectEvents: true
    eventLog: ~/.local/state/opencode-sandbox/runs
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Audit == nil || cfg.Audit.Commands == nil {
		t.Fatal("expected audit.commands config")
	}
	if cfg.Audit.Commands.MaxArgs == nil || *cfg.Audit.Commands.MaxArgs != 32 {
		t.Error("expected command audit maxArgs")
	}
	if cfg.Audit.Commands.IncludeExecutables == nil || len(*cfg.Audit.Commands.IncludeExecutables) != 1 {
		t.Error("expected command audit includeExecutables")
	}
}

func TestMergeEBPFConfig(t *testing.T) {
	base := Defaults()
	overlay := File{
		Network: &Network{
			Backend:       ptr("ebpf"),
			DefaultAction: ptr("allow"),
			EBPF: &EBPF{
				InitImage:           ptr("opencode-sandbox-init:latest"),
				NetworkName:         ptr("opencode-sandbox"),
				EventLog:            ptr("~/.local/state/opencode-sandbox/runs"),
				MirrorProjectEvents: ptr(true),
			},
		},
	}

	merged := MergeEffective(base, overlay)
	if merged.Network.Backend != "ebpf" {
		t.Errorf("expected backend ebpf, got %s", merged.Network.Backend)
	}
	if merged.Network.DefaultAction != "allow" {
		t.Errorf("expected defaultAction allow, got %s", merged.Network.DefaultAction)
	}
	if merged.Network.EBPF.InitImage != "opencode-sandbox-init:latest" {
		t.Errorf("unexpected initImage: %s", merged.Network.EBPF.InitImage)
	}
	if merged.Network.EBPF.NetworkName != "opencode-sandbox" {
		t.Errorf("unexpected networkName: %s", merged.Network.EBPF.NetworkName)
	}
	if !merged.Network.EBPF.MirrorProjectEvents {
		t.Error("expected mirrorProjectEvents true")
	}
}

func TestMergeCommandAuditConfig(t *testing.T) {
	base := Defaults()
	overlay := File{
		Audit: &Audit{
			Commands: &CommandAudit{
				Enabled:             ptr(false),
				FailClosed:          ptr(true),
				MaxArgs:             ptr(12),
				ExcludeExecutables:  &[]string{"/usr/bin/secret-tool"},
				MirrorProjectEvents: ptr(true),
			},
		},
	}

	merged := MergeEffective(base, overlay)
	if merged.Audit.Commands.Enabled {
		t.Error("expected command audit enabled override false")
	}
	if !merged.Audit.Commands.FailClosed {
		t.Error("expected command audit failClosed override true")
	}
	if merged.Audit.Commands.MaxArgs != 12 {
		t.Errorf("unexpected maxArgs: %d", merged.Audit.Commands.MaxArgs)
	}
	if len(merged.Audit.Commands.ExcludeExecutables) != 1 {
		t.Error("expected command audit excludeExecutables override")
	}
	if !merged.Audit.Commands.MirrorProjectEvents {
		t.Error("expected command audit mirrorProjectEvents true")
	}
}
