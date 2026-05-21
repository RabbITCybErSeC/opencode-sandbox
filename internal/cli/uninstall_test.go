package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallCommandIsWrapperCommand(t *testing.T) {
	if !isWrapperCommand("uninstall") {
		t.Fatal("expected uninstall to be a wrapper command")
	}
}

func TestRunUninstallDryRunDoesNotRemoveArtifactsOrRunContainerCommands(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	setupUninstallHome(t, home)
	createUninstallArtifacts(t, home, project)

	var calls [][]string
	captureUninstallCommands(t, &calls, nil)

	if err := runUninstall([]string{"--dry-run"}); err != nil {
		t.Fatalf("runUninstall failed: %v", err)
	}

	assertExists(t, filepath.Join(userConfigRoot(t), "opencode-sandbox"))
	assertExists(t, filepath.Join(home, ".local", "state", "opencode-sandbox"))
	assertExists(t, filepath.Join(home, ".local", "share", "opencode-sandbox-src"))
	assertExists(t, filepath.Join(home, ".local", "bin", "opencode-sandbox"))
	assertExists(t, filepath.Join(project, ".opencode-sandbox.yaml"))
	assertExists(t, filepath.Join(project, ".opencode-sandbox"))
	if len(calls) != 0 {
		t.Fatalf("dry-run should not run container commands, got %v", calls)
	}
}

func TestRunUninstallRemovesGlobalArtifactsAndPreservesProjectArtifacts(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	setupUninstallHome(t, home)
	createUninstallArtifacts(t, home, project)

	containerJSON := []byte(`[
		{"id":"abc123","name":"opencode-sandbox-run"},
		{"id":"def456","name":"other-container"}
	]`)
	var calls [][]string
	captureUninstallCommands(t, &calls, map[string][]byte{
		"container list --all --format json": containerJSON,
	})

	if err := runUninstall(nil); err != nil {
		t.Fatalf("runUninstall failed: %v", err)
	}

	assertNotExists(t, filepath.Join(userConfigRoot(t), "opencode-sandbox"))
	assertNotExists(t, filepath.Join(home, ".local", "state", "opencode-sandbox"))
	assertNotExists(t, filepath.Join(home, ".local", "share", "opencode-sandbox-src"))
	assertNotExists(t, filepath.Join(home, ".local", "bin", "opencode-sandbox"))
	assertExists(t, filepath.Join(project, ".opencode-sandbox.yaml"))
	assertExists(t, filepath.Join(project, ".opencode-sandbox"))

	wantCalls := [][]string{
		{"container", "list", "--all", "--format", "json"},
		{"container", "delete", "--force", "abc123"},
		{"container", "image", "delete", "--force", "ghcr.io/rabbitcybersec/opencode-sandbox:latest"},
		{"container", "image", "delete", "--force", "ghcr.io/rabbitcybersec/opencode-sandbox-init:latest"},
		{"container", "builder", "delete", "--force"},
	}
	assertCalls(t, calls, wantCalls)
}

func TestRunUninstallExplainsProjectArtifactsArePreserved(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	setupUninstallHome(t, home)
	createUninstallArtifacts(t, home, project)

	captureUninstallCommands(t, &[][]string{}, map[string][]byte{
		"container list --all --format json": []byte(`[]`),
	})

	output := captureStdout(t, func() {
		if err := runUninstall(nil); err != nil {
			t.Fatalf("runUninstall failed: %v", err)
		}
	})

	if !strings.Contains(output, "Project-local artifacts are intentionally preserved") {
		t.Fatalf("expected project artifact preservation guidance, got:\n%s", output)
	}
	if !strings.Contains(output, ".opencode-sandbox.yaml") {
		t.Fatalf("expected project config path in guidance, got:\n%s", output)
	}
}

func TestRunUninstallRejectsUnknownOption(t *testing.T) {
	if err := runUninstall([]string{"--surprise"}); err == nil {
		t.Fatal("expected unknown option error")
	}
}

func TestRunUninstallContinuesWhenImageCleanupTimesOut(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	setupUninstallHome(t, home)
	createUninstallArtifacts(t, home, project)

	var calls [][]string
	captureUninstallCommandsWithErrors(t, &calls, nil, map[string]error{
		"container image delete --force ghcr.io/rabbitcybersec/opencode-sandbox:latest": context.DeadlineExceeded,
	})

	if err := runUninstall(nil); err != nil {
		t.Fatalf("runUninstall failed: %v", err)
	}

	assertCallPresent(t, calls, []string{"container", "builder", "delete", "--force"})
}

func setupUninstallHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	mustMkdir(t, filepath.Join(home, "tmp"))
	t.Setenv("TMPDIR", filepath.Join(home, "tmp"))
	t.Setenv("OPENCODE_SANDBOX_BIN", filepath.Join(home, ".local", "bin", "opencode-sandbox"))
	t.Setenv("OPENCODE_SANDBOX_DIR", filepath.Join(home, ".local", "share", "opencode-sandbox-src"))
}

func createUninstallArtifacts(t *testing.T, home, project string) {
	t.Helper()
	mustMkdir(t, filepath.Join(userConfigRoot(t), "opencode-sandbox", "skills"))
	mustMkdir(t, filepath.Join(home, ".local", "state", "opencode-sandbox", "runs"))
	mustMkdir(t, filepath.Join(home, ".local", "share", "opencode-sandbox-src"))
	mustMkdir(t, filepath.Join(home, ".local", "bin"))
	mustWrite(t, filepath.Join(home, ".local", "bin", "opencode-sandbox"), "binary")
	mustWrite(t, filepath.Join(project, ".opencode-sandbox.yaml"), "version: 1\n")
	mustMkdir(t, filepath.Join(project, ".opencode-sandbox", "skills"))
}

func captureUninstallCommands(t *testing.T, calls *[][]string, outputs map[string][]byte) {
	t.Helper()
	captureUninstallCommandsWithErrors(t, calls, outputs, nil)
}

func captureUninstallCommandsWithErrors(t *testing.T, calls *[][]string, outputs map[string][]byte, errors map[string]error) {
	t.Helper()
	oldRun := runUninstallCommand
	runUninstallCommand = func(cmd *exec.Cmd) ([]byte, error) {
		*calls = append(*calls, append([]string(nil), cmd.Args...))
		key := strings.Join(cmd.Args, " ")
		if err, ok := errors[key]; ok {
			return nil, err
		}
		if out, ok := outputs[key]; ok {
			return out, nil
		}
		return nil, nil
	}
	t.Cleanup(func() { runUninstallCommand = oldRun })
}

func assertCalls(t *testing.T, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
			}
		}
	}
}

func assertCallPresent(t *testing.T, calls [][]string, want []string) {
	t.Helper()
	for _, call := range calls {
		if len(call) != len(want) {
			continue
		}
		match := true
		for i := range want {
			if call[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Fatalf("expected call %v in %v", want, calls)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func userConfigRoot(t *testing.T) string {
	t.Helper()
	configRoot, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolving user config dir: %v", err)
	}
	return configRoot
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("closing stdout writer: %v", err)
	}
	os.Stdout = oldStdout

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading stdout: %v", err)
	}
	return string(out)
}
