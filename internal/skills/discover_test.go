package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"skill-a", "skill-b"} {
		d := filepath.Join(dir, name)
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 skills, got %d", len(found))
	}
}
