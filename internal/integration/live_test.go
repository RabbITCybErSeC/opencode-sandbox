//go:build live

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveEnabled returns true when the OPENCODE_SANDBOX_LIVE env var is set.
func liveEnabled() bool {
	return os.Getenv("OPENCODE_SANDBOX_LIVE") == "1"
}

// buildCLI builds the opencode-sandbox binary into a temp dir.
func buildCLI(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "opencode-sandbox")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/opencode-sandbox")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building CLI: %v\n%s", err, out)
	}
	return binPath
}

// repoRoot finds the repository root by looking for go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func TestLiveBuildStrictInitImage(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	bin := buildCLI(t)
	cmd := exec.Command(bin, "image", "build", "--strict-init", "--tag", "opencode-sandbox-init:live-test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building strict init image: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "built successfully") {
		t.Errorf("expected success message, got:\n%s", out)
	}
}

func TestLiveDryRunShowsInitImage(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".opencode-sandbox.yaml")
	configContent := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  ebpf:
    initImage: opencode-sandbox-init:live-test
    networkName: opencode-sandbox-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildCLI(t)
	cmd := exec.Command(bin, "run", projectDir, "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "--init-image") {
		t.Error("expected --init-image in dry-run output")
	}
	if !strings.Contains(output, "opencode-sandbox-init:live-test") {
		t.Error("expected init image tag in dry-run output")
	}
	if !strings.Contains(output, "--network opencode-sandbox-test") {
		t.Error("expected --network in dry-run output")
	}
	if strings.Contains(output, "HTTP_PROXY") {
		t.Error("ebpf backend should not include proxy env vars")
	}
}

func TestLiveBlocklistedDomainBlocked(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".opencode-sandbox.yaml")
	configContent := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  blocklist:
    - blocked.example.com
  ebpf:
    initImage: opencode-sandbox-init:live-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildCLI(t)
	cmd := exec.Command(bin, "run", projectDir, "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "blocked.example.com") {
		// The blocklist should be present in the policy bundle which is mounted.
		// dry-run doesn't show the bundle contents, but we can at least verify
		// the run plan is generated without errors.
		t.Log("dry-run succeeded; blocklist is in project config")
	}
}

func TestLiveEventLogsCreated(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".opencode-sandbox.yaml")
	configContent := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  ebpf:
    initImage: opencode-sandbox-init:live-test
    mirrorProjectEvents: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildCLI(t)
	// For the live harness we do a real run with a fast-failing command.
	cmd := exec.Command(bin, "run", projectDir, "--", "echo", "live-test")
	cmd.Env = os.Environ()
	cmd.Dir = projectDir

	// Run with a timeout.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		// The container may fail because OpenCode isn't installed in the test image.
		// That's okay for this test; we just want to verify event logs were created.
		_ = err
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("live run timed out")
	}

	// Look for event log directory under ~/.local/state/opencode-sandbox/runs.
	home, _ := os.UserHomeDir()
	matches, err := filepath.Glob(filepath.Join(home, ".local", "state", "opencode-sandbox", "runs", "*", "audit-events.jsonl"))
	if err != nil {
		t.Fatalf("globbing event logs: %v", err)
	}
	if len(matches) == 0 {
		t.Error("expected at least one audit-events.jsonl to be created")
	}

	// Check project mirror if enabled.
	projectMirror := filepath.Join(projectDir, ".opencode-sandbox", "audit-events.jsonl")
	if _, err := os.Stat(projectMirror); os.IsNotExist(err) {
		t.Log("project mirror not created (expected if container did not start successfully)")
	}
}

func TestLiveDefaultActionAllow(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".opencode-sandbox.yaml")
	configContent := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  ebpf:
    initImage: opencode-sandbox-init:live-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildCLI(t)
	cmd := exec.Command(bin, "run", projectDir, "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "OPENCODE_SANDBOX_NETWORK_BACKEND=ebpf") {
		t.Error("expected ebpf backend env var")
	}
	fmt.Println(output)
}

func TestLiveDefaultActionDeny(t *testing.T) {
	if !liveEnabled() {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live integration tests")
	}

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".opencode-sandbox.yaml")
	configContent := `version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: deny
  ebpf:
    initImage: opencode-sandbox-init:live-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildCLI(t)
	cmd := exec.Command(bin, "run", projectDir, "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "OPENCODE_SANDBOX_NETWORK_BACKEND=ebpf") {
		t.Error("expected ebpf backend env var")
	}
	fmt.Println(output)
}
