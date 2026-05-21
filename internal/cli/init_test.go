package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitProject(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false); err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	path := filepath.Join(dir, ".opencode-sandbox.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}
}

func TestInitProjectExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".opencode-sandbox.yaml")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false); err == nil {
		t.Fatal("expected error when config exists")
	}
}

func TestInitGlobal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := initGlobal(false); err != nil {
		t.Fatalf("initGlobal failed: %v", err)
	}
}
