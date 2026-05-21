package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"gopkg.in/yaml.v3"
)

func runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: opencode-sandbox config path|show [options]")
	}

	subcmd := args[0]
	switch subcmd {
	case "path":
		return runConfigPath(args[1:])
	case "show":
		return runConfigShow(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", subcmd)
	}
}

func runConfigPath(args []string) error {
	global := false
	project := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--global":
			global = true
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		}
	}

	if global {
		path, err := config.GlobalConfigPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	}

	if project == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		project = wd
	}

	path, err := config.FindProjectConfig(project)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("no project config found for %s", project)
	}
	fmt.Println(path)
	return nil
}

func runConfigShow(args []string) error {
	project := ""
	asJSON := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--json":
			asJSON = true
		}
	}

	if project == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		project = wd
	}

	cfg, err := loadRunConfig(project, RunPlan{})
	if err != nil {
		return err
	}

	// Redact any sensitive values before display.
	cfg = redactConfig(cfg)

	if asJSON {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func redactConfig(cfg config.EffectiveConfig) config.EffectiveConfig {
	// Currently there are no explicit secret fields in the config schema.
	// This function is a hook for future redaction needs.
	return cfg
}
