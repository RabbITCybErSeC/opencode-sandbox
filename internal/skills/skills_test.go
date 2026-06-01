package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverValidSkills(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "skill-a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill-a", "SKILL.md"), []byte("---\nname: Skill A\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "not-a-skill"), 0755); err != nil {
		t.Fatal(err)
	}

	found, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(found))
	}
	if found[0].Name != "Skill A" {
		t.Errorf("expected 'Skill A', got %s", found[0].Name)
	}
}

func TestParseFrontmatter(t *testing.T) {
	data := []byte(`---
name: Test Skill
description: A test skill
---

# Test Skill
`)
	name, desc, err := ParseFrontmatter(data)
	if err != nil {
		t.Fatalf("ParseFrontmatter failed: %v", err)
	}
	if name != "Test Skill" {
		t.Errorf("expected 'Test Skill', got %s", name)
	}
	if desc != "A test skill" {
		t.Errorf("expected 'A test skill', got %s", desc)
	}
}

func TestImportSingle(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: imported\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	opts := ImportOptions{
		Source:  src,
		DestDir: dst,
		Scope:   "global",
		Include: []string{"*"},
	}

	results, err := Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "imported" {
		t.Errorf("expected 'imported', got %s", results[0].Name)
	}

	// Verify file was copied.
	copied := filepath.Join(dst, "imported", "SKILL.md")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("expected copied skill at %s: %v", copied, err)
	}
}

func TestImportSymlinkedSkillFromParent(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(t.TempDir(), "real-skill")
	dst := t.TempDir()

	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("# linked import"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(parent, "linked-import")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	results, err := Import(ImportOptions{Source: parent, DestDir: dst, Scope: "global"})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported skill, got %d", len(results))
	}
	if _, err := os.Stat(filepath.Join(dst, "linked-import", "SKILL.md")); err != nil {
		t.Fatalf("expected copied linked skill: %v", err)
	}
}

func TestImportFilters(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"keep", "skip"} {
		d := filepath.Join(src, name)
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	dst := t.TempDir()
	opts := ImportOptions{
		Source:  src,
		DestDir: dst,
		Scope:   "global",
		Include: []string{"keep"},
	}

	results, err := Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "keep" {
		t.Errorf("expected 'keep', got %s", results[0].Name)
	}
}

func TestImportSymlinkEscape(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(src, "escaped")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	opts := ImportOptions{
		Source:  src,
		DestDir: dst,
		Scope:   "global",
		Include: []string{"*"},
	}

	_, err := Import(opts)
	if err == nil {
		t.Fatal("expected error for symlink escape")
	}
}

func TestImportSymlinkSiblingPrefixEscape(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "skill")
	escape := filepath.Join(parent, "skill-escape")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(escape, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(escape, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(escape, "secret.txt"), filepath.Join(src, "escaped")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	_, err := Import(ImportOptions{
		Source:  src,
		DestDir: dst,
		Scope:   "global",
		Include: []string{"*"},
	})
	if err == nil {
		t.Fatal("expected sibling-prefix symlink escape to be rejected")
	}
}

func TestImportDryRun(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	opts := ImportOptions{
		Source:  src,
		DestDir: dst,
		Scope:   "global",
		DryRun:  true,
		Include: []string{"*"},
	}

	results, err := Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify nothing was copied.
	entries, _ := os.ReadDir(dst)
	if len(entries) > 0 {
		t.Error("expected no files in dry run")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	m := Manifest{
		Version:    1,
		ImportedAt: "2026-05-18T12:00:00Z",
		Skills: []ManifestEntry{
			{Name: "test", Source: "/src", Target: "/dst", Scope: "global", SHA256: "abc123"},
		},
	}

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	read, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}
	if read.Version != 1 {
		t.Errorf("expected version 1, got %d", read.Version)
	}
	if len(read.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(read.Skills))
	}
	if read.Skills[0].Name != "test" {
		t.Errorf("expected 'test', got %s", read.Skills[0].Name)
	}
}
