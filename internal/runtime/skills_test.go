package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func isolateGlobalSkills(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestMergeSkills(t *testing.T) {
	staging := t.TempDir()
	project := t.TempDir()

	// Create global skills in the platform-specific user config location.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	globalSkills := filepath.Join(configDir, "opencode-sandbox", "skills")
	if err := os.MkdirAll(filepath.Join(globalSkills, "global-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalSkills, "global-skill", "SKILL.md"), []byte("global"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create project skills.
	projectSkills := filepath.Join(project, ".opencode-sandbox", "skills")
	if err := os.MkdirAll(filepath.Join(projectSkills, "project-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "project-skill", "SKILL.md"), []byte("project"), 0644); err != nil {
		t.Fatal(err)
	}

	mergedDir, err := MergeSkills(staging, project, config.Defaults().Skills)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}

	// Verify both skills are present.
	if _, err := os.Stat(filepath.Join(mergedDir, "global-skill", "SKILL.md")); err != nil {
		t.Error("expected global skill to be merged")
	}
	if _, err := os.Stat(filepath.Join(mergedDir, "project-skill", "SKILL.md")); err != nil {
		t.Error("expected project skill to be merged")
	}
}

func TestMergeSkillsProjectWins(t *testing.T) {
	staging := t.TempDir()
	project := t.TempDir()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	globalSkills := filepath.Join(configDir, "opencode-sandbox", "skills")
	if err := os.MkdirAll(filepath.Join(globalSkills, "shared"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalSkills, "shared", "SKILL.md"), []byte("global"), 0644); err != nil {
		t.Fatal(err)
	}

	projectSkills := filepath.Join(project, ".opencode-sandbox", "skills")
	if err := os.MkdirAll(filepath.Join(projectSkills, "shared"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "shared", "SKILL.md"), []byte("project"), 0644); err != nil {
		t.Fatal(err)
	}

	mergedDir, err := MergeSkills(staging, project, config.Defaults().Skills)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(mergedDir, "shared", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "project" {
		t.Error("expected project skill to win on conflict")
	}
}

func TestMergeSkillsNoSkills(t *testing.T) {
	isolateGlobalSkills(t)

	staging := t.TempDir()
	project := t.TempDir()

	mergedDir, err := MergeSkills(staging, project, config.Defaults().Skills)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}
	entries, _ := os.ReadDir(mergedDir)
	if len(entries) != 0 {
		t.Errorf("expected empty merged dir, got %d entries", len(entries))
	}
}

func TestMergeSkillsHonorsFiltersAndImportedDir(t *testing.T) {
	isolateGlobalSkills(t)

	staging := t.TempDir()
	project := t.TempDir()
	imported := filepath.Join(project, "custom-skills")
	for _, name := range []string{"keep-one", "drop-one", "keep-secret"} {
		dir := filepath.Join(imported, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Defaults().Skills
	cfg.ImportedDir = "custom-skills"
	cfg.Include = []string{"keep-*"}
	cfg.Exclude = []string{"*-secret"}

	mergedDir, err := MergeSkills(staging, project, cfg)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mergedDir, "keep-one", "SKILL.md")); err != nil {
		t.Error("expected included skill to be merged")
	}
	if _, err := os.Stat(filepath.Join(mergedDir, "drop-one", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("expected non-included skill to be skipped")
	}
	if _, err := os.Stat(filepath.Join(mergedDir, "keep-secret", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("expected excluded skill to be skipped")
	}
}

func TestMergeSkillsIncludesHomeDotConfigGlobalWithProjectImportedDir(t *testing.T) {
	staging := t.TempDir()
	project := t.TempDir()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	globalSkills := filepath.Join(home, ".config", "opencode-sandbox", "skills")
	globalSkill := filepath.Join(globalSkills, "global-xdg")
	if err := os.MkdirAll(globalSkill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalSkill, "SKILL.md"), []byte("global"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults().Skills
	cfg.ImportedDir = ".opencode-sandbox/skills"

	mergedDir, err := MergeSkills(staging, project, cfg)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mergedDir, "global-xdg", "SKILL.md")); err != nil {
		t.Error("expected ~/.config global skill to be merged")
	}
}

func TestMergeSkillsFollowsSymlinkedGlobalSkillDir(t *testing.T) {
	staging := t.TempDir()
	project := t.TempDir()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	target := filepath.Join(t.TempDir(), "real-skill")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("linked"), 0644); err != nil {
		t.Fatal(err)
	}
	globalSkills := filepath.Join(home, ".config", "opencode-sandbox", "skills")
	if err := os.MkdirAll(globalSkills, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(globalSkills, "linked-skill")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	mergedDir, err := MergeSkills(staging, project, config.Defaults().Skills)
	if err != nil {
		t.Fatalf("MergeSkills failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(mergedDir, "linked-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected symlinked global skill to be merged: %v", err)
	}
	if string(data) != "linked" {
		t.Fatalf("unexpected copied skill contents: %q", data)
	}
}
