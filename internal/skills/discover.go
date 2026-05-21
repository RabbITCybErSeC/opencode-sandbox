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
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(root, e.Name())
		skill, err := validateSkillDir(skillDir)
		if err != nil {
			continue // skip invalid
		}
		skills = append(skills, skill)
	}
	return skills, nil
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
