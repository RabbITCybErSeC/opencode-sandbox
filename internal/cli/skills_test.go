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
