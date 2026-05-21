package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

// MergeSkills combines global and project imported skills into a single
// directory suitable for mounting into the sandbox.
func MergeSkills(stagingDir, projectPath string, cfg config.EffectiveSkills) (string, error) {
	mergedDir := filepath.Join(stagingDir, "opencode", "skills")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		return "", fmt.Errorf("creating merged skills dir: %w", err)
	}

	// Copy global imported skills first.
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("getting user config dir: %w", err)
	}
	globalSkills := filepath.Join(configDir, "opencode-sandbox", "skills")
	if err := copySkillsDir(globalSkills, mergedDir, cfg.Include, cfg.Exclude); err != nil {
		return "", fmt.Errorf("copying global skills: %w", err)
	}

	if cfg.ImportedDir != "" {
		importedDir, err := resolveSkillDir(cfg.ImportedDir, projectPath)
		if err != nil {
			return "", err
		}
		if importedDir != globalSkills {
			if err := copySkillsDir(importedDir, mergedDir, cfg.Include, cfg.Exclude); err != nil {
				return "", fmt.Errorf("copying configured skills: %w", err)
			}
		}
	}

	// Copy project imported skills on top (wins on name conflict).
	projectSkills := filepath.Join(projectPath, ".opencode-sandbox", "skills")
	if err := copySkillsDir(projectSkills, mergedDir, cfg.Include, cfg.Exclude); err != nil {
		return "", fmt.Errorf("copying project skills: %w", err)
	}

	return mergedDir, nil
}

func copySkillsDir(src, dst string, include, exclude []string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // no skills to copy
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !skillAllowed(e.Name(), include, exclude) {
			continue
		}
		srcSkill := filepath.Join(src, e.Name())
		dstSkill := filepath.Join(dst, e.Name())
		if err := copyDir(srcSkill, dstSkill); err != nil {
			return fmt.Errorf("copying skill %s: %w", e.Name(), err)
		}
	}
	return nil
}

func resolveSkillDir(path, projectPath string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving skill dir home: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectPath, path)
	}
	return filepath.Clean(path), nil
}

func skillAllowed(name string, include, exclude []string) bool {
	if len(include) > 0 {
		matched := false
		for _, pat := range include {
			if matchSkillGlob(name, pat) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pat := range exclude {
		if matchSkillGlob(name, pat) {
			return false
		}
	}
	return true
}

func matchSkillGlob(name, pat string) bool {
	if pat == "*" {
		return true
	}
	if strings.HasPrefix(pat, "*") && strings.HasSuffix(name, pat[1:]) {
		return true
	}
	if strings.HasSuffix(pat, "*") && strings.HasPrefix(name, strings.TrimSuffix(pat, "*")) {
		return true
	}
	return name == pat
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
