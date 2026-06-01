package skills

import (
	"fmt"
	"os"
	"path/filepath"
)

// SkillInfo describes a discovered skill.
type SkillInfo struct {
	Name        string
	Description string
	Path        string
}

// Discover finds skill directories under root.
func Discover(root string) ([]SkillInfo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var skills []SkillInfo
	for _, e := range entries {
		skillDir, ok, err := skillDirFromEntry(root, e)
		if err != nil || !ok {
			continue
		}
		skill, err := validateSkillDir(skillDir)
		if err != nil {
			continue // skip invalid
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func skillDirFromEntry(root string, e os.DirEntry) (string, bool, error) {
	path := filepath.Join(root, e.Name())
	if e.IsDir() {
		return path, true, nil
	}
	info, err := e.Info()
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", false, nil
	}
	targetInfo, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if !targetInfo.IsDir() {
		return "", false, nil
	}
	return path, true, nil
}

func validateSkillDir(dir string) (SkillInfo, error) {
	skillMd := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillMd); err != nil {
		return SkillInfo{}, fmt.Errorf("missing SKILL.md")
	}
	name := filepath.Base(dir)
	data, err := os.ReadFile(skillMd)
	if err != nil {
		return SkillInfo{Name: name, Path: dir}, nil
	}
	parsedName, desc, _ := ParseFrontmatter(data)
	if parsedName != "" {
		name = parsedName
	}
	return SkillInfo{
		Name:        name,
		Description: desc,
		Path:        dir,
	}, nil
}
