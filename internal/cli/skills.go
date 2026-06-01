package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/skills"
)

func runSkills(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: opencode-sandbox skills import|list [options]")
	}

	subcmd := args[0]
	switch subcmd {
	case "import":
		return runSkillsImport(args[1:])
	case "list":
		return runSkillsList(args[1:])
	default:
		return fmt.Errorf("unknown skills subcommand: %s", subcmd)
	}
}

func runSkillsImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: opencode-sandbox skills import <source> [options]")
	}

	source := args[0]
	scope := "global"
	project := ""
	var include []string
	var exclude []string
	namePrefix := ""
	dryRun := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--scope":
			if i+1 < len(args) {
				scope = args[i+1]
				i++
			}
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--include":
			if i+1 < len(args) {
				include = append(include, args[i+1])
				i++
			}
		case "--exclude":
			if i+1 < len(args) {
				exclude = append(exclude, args[i+1])
				i++
			}
		case "--name-prefix":
			if i+1 < len(args) {
				namePrefix = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		}
	}

	var destDir string
	switch scope {
	case "global":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("getting config dir: %w", err)
		}
		destDir = filepath.Join(configDir, "opencode-sandbox", "skills")
		// On macOS, os.UserConfigDir() returns ~/Library/Preferences,
		// but skills should be installed at ~/.config/opencode-sandbox/skills.
		home, err := os.UserHomeDir()
		if err == nil {
			xdgDir := filepath.Join(home, ".config", "opencode-sandbox", "skills")
			if _, err := os.Stat(xdgDir); err == nil || xdgDir != destDir {
				destDir = xdgDir
			}
		}
	case "project":
		if project == "" {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			project = wd
		}
		destDir = filepath.Join(project, ".opencode-sandbox", "skills")
	default:
		return fmt.Errorf("invalid scope: %s", scope)
	}

	if !dryRun {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("creating dest dir: %w", err)
		}
	}

	opts := skills.ImportOptions{
		Source:     source,
		DestDir:    destDir,
		Scope:      scope,
		NamePrefix: namePrefix,
		Include:    include,
		Exclude:    exclude,
		DryRun:     dryRun,
	}

	results, err := skills.Import(opts)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("Dry run — would import:")
	}
	for _, r := range results {
		fmt.Printf("Imported %s -> %s\n", r.Source, r.Target)
	}
	return nil
}

func runSkillsList(args []string) error {
	jsonOut := false
	project := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			if i+1 >= len(args) {
				return fmt.Errorf("--project requires a path")
			}
			project = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown skills list option: %s", args[i])
		}
	}
	if project == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		project = wd
	}

	list, err := collectSkillsList(project)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}
	for _, item := range list {
		if item.Description != "" {
			fmt.Printf("%s\t%s\t%s\t%s\n", item.Scope, item.Name, item.Path, item.Description)
		} else {
			fmt.Printf("%s\t%s\t%s\n", item.Scope, item.Name, item.Path)
		}
	}
	return nil
}

type skillListItem struct {
	Scope       string `json:"scope"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
}

func collectSkillsList(project string) ([]skillListItem, error) {
	out := []skillListItem{}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("getting config dir: %w", err)
	}

	// On macOS, os.UserConfigDir() returns ~/Library/Preferences,
	// but skills may be installed at ~/.config/opencode-sandbox/skills.
	// Check both locations.
	roots := []struct {
		scope string
		path  string
	}{
		{"global-imported", filepath.Join(configDir, "opencode-sandbox", "skills")},
	}
	if home, err := os.UserHomeDir(); err == nil {
		xdgConfigDir := filepath.Join(home, ".config")
		if xdgConfigDir != configDir {
			roots = append(roots, struct {
				scope string
				path  string
			}{"global-xdg", filepath.Join(xdgConfigDir, "opencode-sandbox", "skills")})
		}
	}
	roots = append(roots, []struct {
		scope string
		path  string
	}{
		{"project-imported", filepath.Join(project, ".opencode-sandbox", "skills")},
		{"project-opencode", filepath.Join(project, ".opencode", "skills")},
		{"project-agents", filepath.Join(project, ".agents", "skills")},
		{"project-claude", filepath.Join(project, ".claude", "skills")},
	}...)
	for _, root := range roots {
		found, err := skills.Discover(root.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("discovering %s skills: %w", root.scope, err)
		}
		for _, s := range found {
			out = append(out, skillListItem{
				Scope:       root.scope,
				Name:        s.Name,
				Description: s.Description,
				Path:        s.Path,
			})
		}
	}
	return out, nil
}
