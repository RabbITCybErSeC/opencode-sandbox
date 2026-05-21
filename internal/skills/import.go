package skills

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportOptions controls skill import behavior.
type ImportOptions struct {
	Source     string
	DestDir    string
	Scope      string
	NamePrefix string
	Include    []string
	Exclude    []string
	DryRun     bool
}

// Import copies a skill or skills from source to destination.
func Import(opts ImportOptions) ([]ManifestEntry, error) {
	info, err := os.Stat(opts.Source)
	if err != nil {
		return nil, fmt.Errorf("source not found: %w", err)
	}

	var sources []string
	if info.IsDir() {
		// Check if source itself is a single skill directory.
		if hasSkillMd(opts.Source) {
			sources = append(sources, opts.Source)
		} else {
			// Source is a directory containing skill subdirectories.
			entries, err := os.ReadDir(opts.Source)
			if err != nil {
				return nil, fmt.Errorf("reading source dir: %w", err)
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				skillDir := filepath.Join(opts.Source, e.Name())
				if hasSkillMd(skillDir) {
					sources = append(sources, skillDir)
				}
			}
		}
	} else {
		return nil, fmt.Errorf("source must be a directory")
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid skills found in %s", opts.Source)
	}

	var results []ManifestEntry
	for _, src := range sources {
		entry, err := importSingle(src, opts)
		if err != nil {
			// Security errors (e.g. symlink escape) must fail hard.
			if isSecurityError(err) {
				return nil, err
			}
			// Skip excluded or invalid skills.
			continue
		}
		results = append(results, entry)
	}

	if !opts.DryRun {
		manifest := Manifest{
			Version:    1,
			ImportedAt: timeNow(),
			Skills:     results,
		}
		manifestPath := filepath.Join(opts.DestDir, "..", "skills-manifest.json")
		if err := WriteManifest(manifestPath, manifest); err != nil {
			return nil, fmt.Errorf("writing manifest: %w", err)
		}
	}

	return results, nil
}

func importSingle(src string, opts ImportOptions) (ManifestEntry, error) {
	data, err := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("reading SKILL.md: %w", err)
	}

	name, _, _ := ParseFrontmatter(data)
	if name == "" {
		name = filepath.Base(src)
	}
	if opts.NamePrefix != "" {
		name = opts.NamePrefix + name
	}

	// Check include/exclude.
	if !matchesFilters(name, opts.Include, opts.Exclude) {
		return ManifestEntry{}, fmt.Errorf("skill %q excluded by filters", name)
	}

	dest := filepath.Join(opts.DestDir, name)

	if !opts.DryRun {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return ManifestEntry{}, fmt.Errorf("creating dest dir: %w", err)
		}
		if err := copySkillDir(src, dest); err != nil {
			return ManifestEntry{}, fmt.Errorf("copying skill: %w", err)
		}
	}

	hash, err := hashSkillDir(src)
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("hashing skill: %w", err)
	}

	return ManifestEntry{
		Name:   name,
		Source: src,
		Target: dest,
		Scope:  opts.Scope,
		SHA256: hash,
	}, nil
}

func isSecurityError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "symlink escapes")
}

func hasSkillMd(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
}

func matchesFilters(name string, include, exclude []string) bool {
	// If include is specified, name must match at least one pattern.
	if len(include) > 0 {
		matched := false
		for _, pat := range include {
			if matchGlob(name, pat) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// Name must not match any exclude pattern.
	for _, pat := range exclude {
		if matchGlob(name, pat) {
			return false
		}
	}
	return true
}

func matchGlob(name, pat string) bool {
	if pat == "*" {
		return true
	}
	if strings.HasPrefix(pat, "*") && strings.HasSuffix(name, pat[1:]) {
		return true
	}
	if strings.HasSuffix(pat, "*") && strings.HasPrefix(name, pat[:len(pat)-1]) {
		return true
	}
	return name == pat
}

func copySkillDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks that escape the source tree.
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			target, _ = filepath.Abs(target)
			srcAbs, _ := filepath.Abs(src)
			relTarget, err := filepath.Rel(srcAbs, target)
			if err != nil {
				return err
			}
			if strings.HasPrefix(relTarget, "..") || filepath.IsAbs(relTarget) {
				return fmt.Errorf("symlink escapes source tree: %s -> %s", path, target)
			}
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

func hashSkillDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
