package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagingDirCreatesLayout(t *testing.T) {
	dir, err := StagingDir()
	if err != nil {
		t.Fatalf("StagingDir failed: %v", err)
	}
	defer os.RemoveAll(dir)

	for _, sub := range []string{"home", "opencode", "logs"} {
		path := filepath.Join(dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("missing subdir %s: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

func TestSafeRemoveAll(t *testing.T) {
	dir, err := StagingDir()
	if err != nil {
		t.Fatalf("StagingDir failed: %v", err)
	}
	if err := SafeRemoveAll(dir); err != nil {
		t.Fatalf("SafeRemoveAll failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected staging dir to be removed")
	}
}

func TestSafeRemoveAllRejectsNonWrapperDir(t *testing.T) {
	dir := t.TempDir()
	if err := SafeRemoveAll(dir); err == nil {
		t.Error("expected SafeRemoveAll to reject non-wrapper directory")
	}
}

func TestResolveProjectPathDefaultsToCwd(t *testing.T) {
	path, err := ResolveProjectPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wd, _ := os.Getwd()
	absWd, _ := filepath.Abs(wd)
	if path != absWd {
		t.Errorf("expected %s, got %s", absWd, path)
	}
}

func TestResolveProjectPathExistingDir(t *testing.T) {
	dir := t.TempDir()
	path, err := ResolveProjectPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absDir, _ := filepath.Abs(dir)
	if path != absDir {
		t.Errorf("expected %s, got %s", absDir, path)
	}
}

func TestResolveProjectPathMissing(t *testing.T) {
	_, err := ResolveProjectPath("/nonexistent/path/12345")
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestResolveProjectPathNotDirectory(t *testing.T) {
	f, err := os.CreateTemp("", "test-file")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	_, err = ResolveProjectPath(f.Name())
	if err == nil {
		t.Error("expected error for file path")
	}
}

func TestCleanupKeep(t *testing.T) {
	dir, _ := StagingDir()
	c := NewCleanup(true)
	if err := c.Remove(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected dir to be preserved with --keep")
	}
	os.RemoveAll(dir) // manual cleanup
}

func TestCleanupRemove(t *testing.T) {
	dir, _ := StagingDir()
	c := NewCleanup(false)
	if err := c.Remove(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected dir to be removed")
	}
}

func TestGenerateRunID(t *testing.T) {
	id1 := GenerateRunID()
	id2 := GenerateRunID()
	if id1 == "" {
		t.Error("expected non-empty runID")
	}
	if id1 == id2 {
		t.Error("expected unique runIDs")
	}
	if !strings.HasPrefix(id1, "opencode-sandbox-") {
		t.Errorf("expected prefix opencode-sandbox-, got %s", id1)
	}
}

func TestProjectName(t *testing.T) {
	if got := ProjectName("/projects/myapp"); got != "myapp" {
		t.Errorf("expected myapp, got %s", got)
	}
	if got := ProjectName("/projects/myapp/"); got != "myapp" {
		t.Errorf("expected myapp, got %s", got)
	}
	if got := ProjectName("myapp"); got != "myapp" {
		t.Errorf("expected myapp, got %s", got)
	}
}
