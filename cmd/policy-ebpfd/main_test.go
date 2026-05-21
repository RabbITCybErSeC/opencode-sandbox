package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicyBundleValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	content := `{
		"version": 1,
		"runId": "run-1",
		"project": {"path": "/p", "name": "proj"},
		"network": {"mode": "strict", "backend": "ebpf", "defaultAction": "allow", "failClosed": true},
		"rules": {"blockDomains": ["*.bad.com"], "allowDomains": ["good.com"]},
		"resolver": {"ttlMinSeconds": 30, "ttlMaxSeconds": 300},
		"events": {"hostJsonl": "/sandbox/logs/events.jsonl", "projectMirrorJsonl": "/workspace/.opencode-sandbox/events.jsonl", "mirrorProjectEvents": false}
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	bundle, err := loadPolicyBundle(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bundle.Version != 1 {
		t.Errorf("expected version 1, got %d", bundle.Version)
	}
	if bundle.RunID != "run-1" {
		t.Errorf("unexpected runId: %s", bundle.RunID)
	}
	if bundle.Network.Mode != "strict" {
		t.Errorf("unexpected mode: %s", bundle.Network.Mode)
	}
	if bundle.Network.Backend != "ebpf" {
		t.Errorf("unexpected backend: %s", bundle.Network.Backend)
	}
	if len(bundle.Rules.BlockDomains) != 1 || bundle.Rules.BlockDomains[0] != "*.bad.com" {
		t.Errorf("unexpected blockDomains: %v", bundle.Rules.BlockDomains)
	}
}

func TestLoadPolicyBundleMissingFile(t *testing.T) {
	_, err := loadPolicyBundle("/nonexistent/policy.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadPolicyBundleInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadPolicyBundle(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateBundleUnsupportedVersion(t *testing.T) {
	bundle := &PolicyBundle{Version: 2}
	if err := validateBundle(bundle); err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestValidateBundleWrongBackend(t *testing.T) {
	bundle := &PolicyBundle{
		Version: 1,
		Network: struct {
			Mode          string `json:"mode"`
			Backend       string `json:"backend"`
			DefaultAction string `json:"defaultAction"`
			FailClosed    bool   `json:"failClosed"`
		}{
			Mode:    "strict",
			Backend: "proxy",
		},
	}
	if err := validateBundle(bundle); err == nil {
		t.Error("expected error for non-ebpf backend")
	}
}

func TestValidateBundleUnsupportedMode(t *testing.T) {
	bundle := &PolicyBundle{
		Version: 1,
		Network: struct {
			Mode          string `json:"mode"`
			Backend       string `json:"backend"`
			DefaultAction string `json:"defaultAction"`
			FailClosed    bool   `json:"failClosed"`
		}{
			Mode:    "practical",
			Backend: "ebpf",
		},
	}
	if err := validateBundle(bundle); err == nil {
		t.Error("expected error for practical mode with ebpf backend")
	}
}

func TestRunFailClosedMissingPolicy(t *testing.T) {
	// Point to a nonexistent policy file and set fail-closed behavior.
	tmpDir := t.TempDir()
	t.Setenv("OPENCODE_SANDBOX_POLICY_FILE", filepath.Join(tmpDir, "missing.json"))

	err := run()
	if err == nil {
		t.Fatal("expected error when policy file is missing")
	}
}

func TestRunFailClosedWithAttachFailure(t *testing.T) {
	// On macOS cgroup2 is unavailable, so attach will fail.
	// With failClosed=true the daemon should error out.
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.json")
	hostEvents := filepath.Join(dir, "logs", "network-events.jsonl")
	content := `{
		"version": 1,
		"runId": "run-1",
		"project": {"path": "/p", "name": "proj"},
		"network": {"mode": "strict", "backend": "ebpf", "defaultAction": "allow", "failClosed": true},
		"rules": {"blockDomains": [], "allowDomains": []},
		"resolver": {"ttlMinSeconds": 30, "ttlMaxSeconds": 300},
		"events": {"hostJsonl": "` + hostEvents + `", "projectMirrorJsonl": "", "mirrorProjectEvents": false}
	}`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_SANDBOX_POLICY_FILE", policyPath)

	err := run()
	if err == nil {
		t.Fatal("expected error when attach fails with failClosed=true")
	}
}

func TestRunNoFailClosedWithAttachFailure(t *testing.T) {
	// On macOS cgroup2 is unavailable, so attach will fail.
	// With failClosed=false the daemon should continue with a warning.
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.json")
	hostEvents := filepath.Join(dir, "logs", "network-events.jsonl")
	content := `{
		"version": 1,
		"runId": "run-1",
		"project": {"path": "/p", "name": "proj"},
		"network": {"mode": "strict", "backend": "ebpf", "defaultAction": "allow", "failClosed": false},
		"rules": {"blockDomains": [], "allowDomains": []},
		"resolver": {"ttlMinSeconds": 30, "ttlMaxSeconds": 300},
		"events": {"hostJsonl": "` + hostEvents + `", "projectMirrorJsonl": "", "mirrorProjectEvents": false}
	}`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_SANDBOX_POLICY_FILE", policyPath)

	err := run()
	if err != nil {
		t.Fatalf("expected success when attach fails with failClosed=false, got: %v", err)
	}
}

func TestProbeCapabilitiesOnMacOS(t *testing.T) {
	caps := probeCapabilities()
	// On macOS cgroup2 should not be available.
	if caps.Cgroup2Available {
		t.Log("cgroup2 is unexpectedly available on this system")
	}
}
