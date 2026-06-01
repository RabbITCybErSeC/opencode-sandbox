package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectSkillsList(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(configDir, "opencode-sandbox", "skills", "global")
	projectImported := filepath.Join(project, ".opencode-sandbox", "skills", "project")
	projectNative := filepath.Join(project, ".opencode", "skills", "native")
	for _, dir := range []string{global, projectImported, projectNative} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(dir)+"\ndescription: test\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	list, err := collectSkillsList(project)
	if err != nil {
		t.Fatalf("collectSkillsList failed: %v", err)
	}
	got := map[string]string{}
	for _, item := range list {
		got[item.Name] = item.Scope
	}
	if got["global"] != "global-imported" {
		t.Fatalf("expected global imported skill, got %v", got)
	}
	if got["project"] != "project-imported" {
		t.Fatalf("expected project imported skill, got %v", got)
	}
	if got["native"] != "project-opencode" {
		t.Fatalf("expected project native skill, got %v", got)
	}
}

func TestCollectSkillsListIncludesHomeDotConfigGlobal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	global := filepath.Join(home, ".config", "opencode-sandbox", "skills", "global-xdg")
	if err := os.MkdirAll(global, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "SKILL.md"), []byte("---\nname: global-xdg\ndescription: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	list, err := collectSkillsList(project)
	if err != nil {
		t.Fatalf("collectSkillsList failed: %v", err)
	}
	for _, item := range list {
		if item.Name == "global-xdg" && item.Scope == "global-xdg" {
			return
		}
	}
	t.Fatalf("expected ~/.config global skill in list, got %#v", list)
}

func TestCollectSkillsListIncludesSymlinkedHomeDotConfigGlobal(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	target := filepath.Join(t.TempDir(), "real-skill")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\nname: linked-global\ndescription: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	globalRoot := filepath.Join(home, ".config", "opencode-sandbox", "skills")
	if err := os.MkdirAll(globalRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(globalRoot, "linked-global")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	list, err := collectSkillsList(project)
	if err != nil {
		t.Fatalf("collectSkillsList failed: %v", err)
	}
	for _, item := range list {
		if item.Name == "linked-global" && item.Scope == "global-xdg" {
			return
		}
	}
	t.Fatalf("expected symlinked ~/.config global skill in list, got %#v", list)
}

func TestRunSkillsImportGlobalUsesHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	source := filepath.Join(t.TempDir(), "skill")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: imported-global\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runSkillsImport([]string{source}); err != nil {
		t.Fatalf("runSkillsImport failed: %v", err)
	}

	want := filepath.Join(home, ".config", "opencode-sandbox", "skills", "imported-global", "SKILL.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected global skill at %s: %v", want, err)
	}
}
