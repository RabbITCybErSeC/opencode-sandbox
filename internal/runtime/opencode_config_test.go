package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateOpenCodeConfig(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateOpenCodeConfig(dir); err != nil {
		t.Fatalf("GenerateOpenCodeConfig failed: %v", err)
	}
	path := filepath.Join(dir, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected overlay config at %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), `"autoupdate": false`) {
		t.Errorf("expected autoupdate false in overlay, got:\n%s", string(data))
	}
}

func TestStageOpenCodeConfigFromCopiesTUI(t *testing.T) {
	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "tui.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := t.TempDir()
	if err := StageOpenCodeConfigFrom(sandbox, hostDir); err != nil {
		t.Fatalf("StageOpenCodeConfigFrom failed: %v", err)
	}

	copied := filepath.Join(sandbox, ".config", "opencode", "tui.json")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("expected tui.json to be copied: %v", err)
	}
}

func TestStageOpenCodeConfigFromGeneratesOverlay(t *testing.T) {
	hostDir := t.TempDir()
	sandbox := t.TempDir()
	if err := StageOpenCodeConfigFrom(sandbox, hostDir); err != nil {
		t.Fatalf("StageOpenCodeConfigFrom failed: %v", err)
	}

	overlay := filepath.Join(sandbox, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(overlay); err != nil {
		t.Errorf("expected overlay config: %v", err)
	}
}

func TestStageOpenCodeConfigFromDoesNotModifyHost(t *testing.T) {
	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "tui.json"), []byte(`{"original":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox := t.TempDir()
	if err := StageOpenCodeConfigFrom(sandbox, hostDir); err != nil {
		t.Fatalf("StageOpenCodeConfigFrom failed: %v", err)
	}

	// Verify host file is unchanged.
	data, err := os.ReadFile(filepath.Join(hostDir, "tui.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), `"original":true`) {
		t.Error("host config was modified")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
