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

func TestDiscoverFollowsSymlinkedSkillDirs(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "real-skill")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\nname: linked-skill\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked-skill")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(found))
	}
	if found[0].Name != "linked-skill" {
		t.Fatalf("expected linked skill, got %#v", found[0])
	}
}
