package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func TestFindContainerfileInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	found := findContainerfile(false)
	if found == "" {
		t.Fatal("expected to find Containerfile")
	}
	if filepath.Base(found) != "Containerfile" {
		t.Errorf("expected Containerfile, got %s", filepath.Base(found))
	}
}

func TestFindContainerfileNotFound(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	found := findContainerfile(false)
	if found != "" {
		t.Errorf("expected no Containerfile, got %s", found)
	}
}

func TestResolveBuildSourceUsesExplicitFileAndContext(t *testing.T) {
	dir := t.TempDir()
	contextDir := filepath.Join(dir, "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatal(err)
	}
	containerfile := filepath.Join(dir, "Customfile")
	if err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	source, err := resolveBuildSource(false, containerfile, contextDir)
	if err != nil {
		t.Fatalf("resolveBuildSource failed: %v", err)
	}
	if source.Containerfile != containerfile {
		t.Fatalf("containerfile = %q, want %q", source.Containerfile, containerfile)
	}
	if source.ContextDir != contextDir {
		t.Fatalf("context = %q, want %q", source.ContextDir, contextDir)
	}
}

func TestResolveBuildSourceUsesExplicitContext(t *testing.T) {
	dir := t.TempDir()
	containerfile := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	source, err := resolveBuildSource(false, "", dir)
	if err != nil {
		t.Fatalf("resolveBuildSource failed: %v", err)
	}
	if source.Containerfile != containerfile {
		t.Fatalf("containerfile = %q, want %q", source.Containerfile, containerfile)
	}
	if source.ContextDir != dir {
		t.Fatalf("context = %q, want %q", source.ContextDir, dir)
	}
}

func TestResolveBuildSourceUsesEnvDirBeforeCurrentTree(t *testing.T) {
	envDir := t.TempDir()
	envContainerfile := filepath.Join(envDir, "Containerfile")
	if err := os.WriteFile(envContainerfile, []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	currentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(currentDir, "Containerfile"), []byte("FROM debian\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(currentDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)
	t.Setenv("OPENCODE_SANDBOX_DIR", envDir)

	source, err := resolveBuildSource(false, "", "")
	if err != nil {
		t.Fatalf("resolveBuildSource failed: %v", err)
	}
	if source.Containerfile != envContainerfile {
		t.Fatalf("containerfile = %q, want %q", source.Containerfile, envContainerfile)
	}
}

func TestResolveBuildSourceUsesInstalledDirBeforeCurrentTree(t *testing.T) {
	installedDir := t.TempDir()
	installedContainerfile := filepath.Join(installedDir, "Containerfile")
	if err := os.WriteFile(installedContainerfile, []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	currentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(currentDir, "Containerfile"), []byte("FROM debian\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(currentDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	oldInstalled := installedSourceDir
	installedSourceDir = installedDir
	defer func() { installedSourceDir = oldInstalled }()

	source, err := resolveBuildSource(false, "", "")
	if err != nil {
		t.Fatalf("resolveBuildSource failed: %v", err)
	}
	if source.Containerfile != installedContainerfile {
		t.Fatalf("containerfile = %q, want %q", source.Containerfile, installedContainerfile)
	}
}

func TestFindContainerfileInitInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Containerfile.init"), []byte("FROM debian:bookworm-slim\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	found := findContainerfile(true)
	if found == "" {
		t.Fatal("expected to find Containerfile.init")
	}
	if filepath.Base(found) != "Containerfile.init" {
		t.Errorf("expected Containerfile.init, got %s", filepath.Base(found))
	}
}

func TestFindContainerfileInitNotFound(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	found := findContainerfile(true)
	if found != "" {
		t.Errorf("expected no Containerfile.init, got %s", found)
	}
}

func TestDefaultImageTag(t *testing.T) {
	if got := defaultImageTag(false); got != config.DefaultImageName {
		t.Fatalf("default app image tag = %q", got)
	}
	if got := defaultImageTag(true); got != config.DefaultStrictInitImage {
		t.Fatalf("default strict init image tag = %q", got)
	}
}

func TestRunImagePullDefaultRuntime(t *testing.T) {
	calls := captureImageCommands(t)
	if err := runImagePull(nil); err != nil {
		t.Fatalf("runImagePull failed: %v", err)
	}
	want := []string{"container", "image", "pull", config.DefaultImageName}
	assertArgv(t, calls, want)
}

func TestRunImagePullStrictInit(t *testing.T) {
	calls := captureImageCommands(t)
	if err := runImagePull([]string{"--strict-init"}); err != nil {
		t.Fatalf("runImagePull failed: %v", err)
	}
	want := []string{"container", "image", "pull", config.DefaultStrictInitImage}
	assertArgv(t, calls, want)
}

func TestRunImagePullCustomTag(t *testing.T) {
	calls := captureImageCommands(t)
	if err := runImagePull([]string{"--tag", "example.test/custom:dev"}); err != nil {
		t.Fatalf("runImagePull failed: %v", err)
	}
	want := []string{"container", "image", "pull", "example.test/custom:dev"}
	assertArgv(t, calls, want)
}

func TestRunImagePullPropagatesCommandError(t *testing.T) {
	oldRun := runImageCommand
	runImageCommand = func(cmd *exec.Cmd) error {
		return fmt.Errorf("boom")
	}
	defer func() { runImageCommand = oldRun }()

	if err := runImagePull(nil); err == nil {
		t.Fatal("expected command error")
	}
}

func TestRunImageBuildUsesExplicitContextAndPublishedDefaultTag(t *testing.T) {
	dir := t.TempDir()
	containerfile := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	calls := captureImageCommands(t)
	if err := runImageBuild([]string{"--context", dir}); err != nil {
		t.Fatalf("runImageBuild failed: %v", err)
	}
	want := []string{
		"container",
		"build",
		"--file", containerfile,
		"--tag", config.DefaultImageName,
		dir,
	}
	assertArgv(t, calls, want)
}

func captureImageCommands(t *testing.T) *[][]string {
	t.Helper()
	var calls [][]string
	oldRun := runImageCommand
	runImageCommand = func(cmd *exec.Cmd) error {
		calls = append(calls, append([]string(nil), cmd.Args...))
		return nil
	}
	t.Cleanup(func() { runImageCommand = oldRun })
	return &calls
}

func assertArgv(t *testing.T, got *[][]string, want []string) {
	t.Helper()
	if len(*got) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*got))
	}
	call := (*got)[0]
	if len(call) != len(want) {
		t.Fatalf("argv = %v, want %v", call, want)
	}
	for i := range want {
		if call[i] != want[i] {
			t.Fatalf("argv = %v, want %v", call, want)
		}
	}
}
