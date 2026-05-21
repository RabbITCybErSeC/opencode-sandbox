package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/containercmd"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"help command", []string{"help"}, false},
		{"doctor command", []string{"doctor"}, false},
		{"unknown forwarded", []string{"--help", "--dry-run"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "doctor command" {
				withFakeDoctorEnvironment(t)
			}
			err := Execute(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func withFakeDoctorEnvironment(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	binDir := t.TempDir()
	containerPath := filepath.Join(binDir, "container")
	if err := os.WriteFile(containerPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("writing fake container binary: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecuteRootHelpForwardsToRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldRunContainer := runContainer
	oldInspectRunImage := inspectRunImage
	defer func() {
		runContainer = oldRunContainer
		inspectRunImage = oldInspectRunImage
	}()

	var argv []string
	runContainer = func(args []string) error {
		argv = append([]string(nil), args...)
		return nil
	}
	inspectRunImage = func(image string) ([]byte, error) {
		return nil, nil
	}

	if err := Execute([]string{"--help"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(argv) == 0 {
		t.Fatal("expected container runner to be called")
	}
	if argv[len(argv)-1] != "--help" {
		t.Fatalf("expected --help forwarded to OpenCode, got argv %v", argv)
	}
}

func TestPreflightContainerImagesChecksRuntimeAndInitImages(t *testing.T) {
	oldInspectRunImage := inspectRunImage
	defer func() { inspectRunImage = oldInspectRunImage }()

	cfg := config.Defaults()
	cfg.Audit.Commands.Enabled = true
	plan := containercmd.Plan{
		Image:     "runtime:test",
		Effective: cfg,
	}
	var images []string
	inspectRunImage = func(image string) ([]byte, error) {
		images = append(images, image)
		return nil, nil
	}

	if err := preflightContainerImages(plan); err != nil {
		t.Fatalf("preflightContainerImages failed: %v", err)
	}
	want := []string{"runtime:test", config.DefaultStrictInitImage}
	if len(images) != len(want) {
		t.Fatalf("inspected images = %v, want %v", images, want)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("inspected images = %v, want %v", images, want)
		}
	}
}

func TestPreflightContainerImagesReportsMissingRuntimeImage(t *testing.T) {
	oldInspectRunImage := inspectRunImage
	defer func() { inspectRunImage = oldInspectRunImage }()

	inspectRunImage = func(image string) ([]byte, error) {
		return []byte("Image not found"), errors.New("exit status 1")
	}
	err := preflightContainerImages(containercmd.Plan{
		Image:     "runtime:test",
		Effective: config.Defaults(),
	})
	if err == nil {
		t.Fatal("expected missing image error")
	}
	if !strings.Contains(err.Error(), "opencode-sandbox image pull") {
		t.Fatalf("expected pull guidance, got %v", err)
	}
}

func TestPreflightContainerImagesReportsMissingInitImage(t *testing.T) {
	oldInspectRunImage := inspectRunImage
	defer func() { inspectRunImage = oldInspectRunImage }()

	inspectRunImage = func(image string) ([]byte, error) {
		if image == "runtime:test" {
			return nil, nil
		}
		return []byte("Image not found"), errors.New("exit status 1")
	}
	cfg := config.Defaults()
	cfg.Audit.Commands.Enabled = true
	err := preflightContainerImages(containercmd.Plan{
		Image:     "runtime:test",
		Effective: cfg,
	})
	if err == nil {
		t.Fatal("expected missing init image error")
	}
	if !strings.Contains(err.Error(), "opencode-sandbox image pull --strict-init") {
		t.Fatalf("expected strict init pull guidance, got %v", err)
	}
}

func TestBuildRunContainerPlanDryRunDoesNotMaterialize(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	_, cleanup, err := buildRunContainerPlan(RunPlan{ProjectPath: project, OpenCodeArgs: []string{"--help"}}, effectiveRunOptions{Materialize: false})
	if err != nil {
		t.Fatalf("buildRunContainerPlan failed: %v", err)
	}
	if cleanup != nil {
		t.Fatal("dry-run plan should not return cleanup for materialized staging")
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "opencode-sandbox")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create durable state, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".opencode-sandbox")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create project state, stat err=%v", err)
	}
}

func TestIsWrapperCommand(t *testing.T) {
	for cmd := range wrapperCommands {
		if !isWrapperCommand(cmd) {
			t.Errorf("expected %q to be a wrapper command", cmd)
		}
	}
	if isWrapperCommand("opencode") {
		t.Error("expected 'opencode' not to be a wrapper command")
	}
}

func TestParseRunArgsEmpty(t *testing.T) {
	plan, err := parseRunArgs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.ProjectPath == "" {
		t.Error("expected project path to default to cwd")
	}
	if len(plan.OpenCodeArgs) != 0 {
		t.Errorf("expected no opencode args, got %v", plan.OpenCodeArgs)
	}
}

func TestParseRunArgsDirectory(t *testing.T) {
	dir := t.TempDir()
	plan, err := parseRunArgs([]string{dir, "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absDir, _ := filepath.Abs(dir)
	if plan.ProjectPath != absDir {
		t.Errorf("expected project path %s, got %s", absDir, plan.ProjectPath)
	}
	if len(plan.OpenCodeArgs) != 1 || plan.OpenCodeArgs[0] != "--help" {
		t.Errorf("expected [--help], got %v", plan.OpenCodeArgs)
	}
}

func TestParseRunArgsNoDirectory(t *testing.T) {
	plan, err := parseRunArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.ProjectPath == "" {
		t.Error("expected project path to default to cwd")
	}
	if len(plan.OpenCodeArgs) != 1 || plan.OpenCodeArgs[0] != "--help" {
		t.Errorf("expected [--help], got %v", plan.OpenCodeArgs)
	}
}

func TestParseRunArgsEscapeHatch(t *testing.T) {
	plan, err := parseRunArgs([]string{".", "--", "run", "prompt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wd, _ := os.Getwd()
	absWd, _ := filepath.Abs(wd)
	if plan.ProjectPath != absWd {
		t.Errorf("expected project path %s, got %s", absWd, plan.ProjectPath)
	}
	want := []string{"run", "prompt"}
	if len(plan.OpenCodeArgs) != len(want) {
		t.Fatalf("expected %v, got %v", want, plan.OpenCodeArgs)
	}
	for i := range want {
		if plan.OpenCodeArgs[i] != want[i] {
			t.Errorf("arg %d: expected %q, got %q", i, want[i], plan.OpenCodeArgs[i])
		}
	}
}

func TestParseRunArgsRunPrompt(t *testing.T) {
	plan, err := parseRunArgs([]string{"run", "summarize this repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.ProjectPath == "" {
		t.Error("expected project path to default to cwd")
	}
	want := []string{"run", "summarize this repo"}
	if len(plan.OpenCodeArgs) != len(want) {
		t.Fatalf("expected %v, got %v", want, plan.OpenCodeArgs)
	}
}

func TestParseRunArgsRelativeDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	// Change to parent so "child" resolves as a relative directory.
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	plan, err := parseRunArgs([]string{"child", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absChild, _ := filepath.Abs(child)
	// macOS may add /private prefix; resolve both through EvalSymlinks.
	resolvedPlan, _ := filepath.EvalSymlinks(plan.ProjectPath)
	resolvedChild, _ := filepath.EvalSymlinks(absChild)
	if resolvedPlan != resolvedChild {
		t.Errorf("expected project path %s, got %s", resolvedChild, resolvedPlan)
	}
}
