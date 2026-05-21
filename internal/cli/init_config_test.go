package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProjectCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false); err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	path := filepath.Join(dir, ".opencode-sandbox.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "version: 1") {
		t.Error("expected version: 1 in config")
	}
	if !strings.Contains(string(data), "localhostAccess:") {
		t.Error("expected localhostAccess in generated project config")
	}
	if !strings.Contains(string(data), "audit:") || !strings.Contains(string(data), "commands:") {
		t.Error("expected command audit in generated project config")
	}
}

func TestInitProjectRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".opencode-sandbox.yaml")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false); err == nil {
		t.Fatal("expected error when config exists")
	}
}

func TestInitProjectForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".opencode-sandbox.yaml")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, true); err != nil {
		t.Fatalf("initProject --force failed: %v", err)
	}
}

func TestInitGlobalCreatesConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := initGlobal(false); err != nil {
		t.Fatalf("initGlobal failed: %v", err)
	}
}

func TestDetectExistingConfigs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	comments := detectExistingConfigs(dir)
	if !strings.Contains(comments, "opencode.json") {
		t.Error("expected opencode.json to be detected")
	}
}

func TestConfigPathGlobal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := runConfigPath([]string{"--global"}); err != nil {
		t.Fatalf("config path --global failed: %v", err)
	}
}

func TestConfigPathProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".opencode-sandbox.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runConfigPath([]string{"--project", dir}); err != nil {
		t.Fatalf("config path --project failed: %v", err)
	}
}

func TestConfigShow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".opencode-sandbox.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	if err := runConfigShow([]string{}); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
}

func TestConfigShowJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".opencode-sandbox.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	if err := runConfigShow([]string{"--json"}); err != nil {
		t.Fatalf("config show --json failed: %v", err)
	}
}
