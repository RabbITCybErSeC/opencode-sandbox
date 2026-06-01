package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteUpgradeDispatchesWrapperCommand(t *testing.T) {
	withUpgradeWorkingDir(t, "")
	calls := captureUpgradeCommands(t, nil)

	if err := Execute([]string{"upgrade", "v1.15.13"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	assertUpgradeCommands(t, calls, "ghcr.io/rabbitcybersec/opencode-sandbox:latest", "v1.15.13")
}

func TestRunUpgradeDefaultsToLatest(t *testing.T) {
	withUpgradeWorkingDir(t, "")
	calls := captureUpgradeCommands(t, nil)

	if err := runUpgrade(nil); err != nil {
		t.Fatalf("runUpgrade failed: %v", err)
	}

	assertUpgradeCommands(t, calls, "ghcr.io/rabbitcybersec/opencode-sandbox:latest", "latest")
}

func TestRunUpgradeUsesConfiguredImageAndRestoresUnprivilegedUser(t *testing.T) {
	withUpgradeWorkingDir(t, "version: 1\nimage:\n  name: example.test/opencode-sandbox:custom\n")
	var containerfile string
	calls := captureUpgradeCommands(t, func(cmd *exec.Cmd) {
		if len(cmd.Args) > 1 && cmd.Args[1] == "build" {
			path := argValue(t, cmd.Args, "--file")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading generated Containerfile: %v", err)
			}
			containerfile = string(data)
		}
	})

	if err := runUpgrade([]string{"v1.15.13"}); err != nil {
		t.Fatalf("runUpgrade failed: %v", err)
	}

	assertUpgradeCommands(t, calls, "example.test/opencode-sandbox:custom", "v1.15.13")
	for _, want := range []string{
		"ARG BASE_IMAGE",
		"FROM ${BASE_IMAGE}",
		"USER root",
		"ARG OPENCODE_VERSION=latest",
		`RUN npm install -g "opencode-ai@${OPENCODE_VERSION}" && npm cache clean --force`,
		"USER opencode",
	} {
		if !strings.Contains(containerfile, want) {
			t.Fatalf("generated Containerfile missing %q:\n%s", want, containerfile)
		}
	}
}

func TestRunUpgradeRejectsMethodFlag(t *testing.T) {
	withUpgradeWorkingDir(t, "")

	err := runUpgrade([]string{"--method", "npm"})
	if err == nil || !strings.Contains(err.Error(), "--method") {
		t.Fatalf("expected --method rejection, got %v", err)
	}
}

func TestRunUpgradeRejectsExtraArguments(t *testing.T) {
	withUpgradeWorkingDir(t, "")

	err := runUpgrade([]string{"v1.15.13", "unexpected"})
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestRunUpgradeStopsWhenPullFails(t *testing.T) {
	withUpgradeWorkingDir(t, "")
	var calls [][]string
	oldRun := runUpgradeCommand
	runUpgradeCommand = func(cmd *exec.Cmd) error {
		calls = append(calls, append([]string(nil), cmd.Args...))
		return errors.New("pull failed")
	}
	t.Cleanup(func() { runUpgradeCommand = oldRun })

	err := runUpgrade(nil)
	if err == nil || !strings.Contains(err.Error(), "pulling runtime image") {
		t.Fatalf("expected pull error, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected build to be skipped, got calls %v", calls)
	}
}

func TestRunUpgradeReportsBuildFailure(t *testing.T) {
	withUpgradeWorkingDir(t, "")
	var calls [][]string
	oldRun := runUpgradeCommand
	runUpgradeCommand = func(cmd *exec.Cmd) error {
		calls = append(calls, append([]string(nil), cmd.Args...))
		if len(cmd.Args) > 1 && cmd.Args[1] == "build" {
			return errors.New("build failed")
		}
		return nil
	}
	t.Cleanup(func() { runUpgradeCommand = oldRun })

	err := runUpgrade(nil)
	if err == nil || !strings.Contains(err.Error(), "building upgraded runtime image") {
		t.Fatalf("expected build error, got %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected pull and build calls, got %v", calls)
	}
}

func withUpgradeWorkingDir(t *testing.T, configYAML string) {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(project, ".opencode-sandbox.yaml"), []byte(configYAML), 0644); err != nil {
			t.Fatalf("writing project config: %v", err)
		}
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatalf("changing working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
}

func captureUpgradeCommands(t *testing.T, inspect func(*exec.Cmd)) *[][]string {
	t.Helper()
	var calls [][]string
	oldRun := runUpgradeCommand
	runUpgradeCommand = func(cmd *exec.Cmd) error {
		calls = append(calls, append([]string(nil), cmd.Args...))
		if inspect != nil {
			inspect(cmd)
		}
		return nil
	}
	t.Cleanup(func() { runUpgradeCommand = oldRun })
	return &calls
}

func assertUpgradeCommands(t *testing.T, calls *[][]string, image, target string) {
	t.Helper()
	if len(*calls) != 2 {
		t.Fatalf("expected pull and build calls, got %v", *calls)
	}
	wantPull := []string{"container", "image", "pull", image}
	assertStringSlice(t, (*calls)[0], wantPull)

	build := (*calls)[1]
	if len(build) == 0 || build[0] != "container" || build[1] != "build" {
		t.Fatalf("unexpected build command: %v", build)
	}
	if got := argValue(t, build, "--tag"); got != image {
		t.Fatalf("build tag = %q, want %q", got, image)
	}
	if !containsPair(build, "--build-arg", "BASE_IMAGE="+image) {
		t.Fatalf("build command missing base image arg: %v", build)
	}
	if !containsPair(build, "--build-arg", "OPENCODE_VERSION="+target) {
		t.Fatalf("build command missing OpenCode version arg: %v", build)
	}
}

func argValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	t.Fatalf("missing %s in %v", flag, args)
	return ""
}

func containsPair(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
