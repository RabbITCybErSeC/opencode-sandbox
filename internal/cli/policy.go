package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/network"
)

func runPolicy(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: opencode-sandbox policy test <domain-or-url>")
	}

	subcmd := args[0]
	switch subcmd {
	case "test":
		return runPolicyTest(args[1:])
	default:
		return fmt.Errorf("unknown policy subcommand: %s", subcmd)
	}
}

func runPolicyTest(args []string) error {
	var target string
	projectPath := ""
	asJSON := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				projectPath = args[i+1]
				i++
			}
		case "--json":
			asJSON = true
		default:
			if target == "" && !strings.HasPrefix(args[i], "-") {
				target = args[i]
			}
		}
	}

	if target == "" {
		return fmt.Errorf("usage: opencode-sandbox policy test <domain-or-url> [--project <path>] [--json]")
	}

	hostname, err := network.ExtractHostname(target)
	if err != nil {
		return fmt.Errorf("parsing target: %w", err)
	}

	cfg, err := loadPolicyConfig(projectPath)
	if err != nil {
		return err
	}

	engine := network.NewEngine(cfg)
	decision := engine.Test(hostname)

	if asJSON {
		data, _ := json.MarshalIndent(decision, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	status := "allowed"
	if decision.Blocked {
		status = "blocked"
	}
	fmt.Printf("%s: %s\n", status, hostname)
	if decision.MatchedRule != "" {
		fmt.Printf("matched rule: %s\n", decision.MatchedRule)
	}
	fmt.Printf("mode: %s\n", decision.Mode)
	if decision.Note != "" {
		fmt.Printf("note: %s\n", decision.Note)
	}
	return nil
}

func loadPolicyConfig(projectPath string) (config.EffectiveConfig, error) {
	base := config.Defaults()

	globalPath, err := config.GlobalConfigPath()
	if err != nil {
		return config.EffectiveConfig{}, err
	}

	if _, err := os.Stat(globalPath); err == nil {
		globalCfg, err := config.Load(globalPath)
		if err != nil {
			return config.EffectiveConfig{}, fmt.Errorf("loading global config: %w", err)
		}
		base = config.MergeEffective(base, globalCfg)
	}

	if projectPath != "" {
		projectCfgPath, err := config.FindProjectConfig(projectPath)
		if err != nil {
			return config.EffectiveConfig{}, err
		}
		if projectCfgPath != "" {
			projectCfg, err := config.Load(projectCfgPath)
			if err != nil {
				return config.EffectiveConfig{}, fmt.Errorf("loading project config: %w", err)
			}
			base = config.MergeEffective(base, projectCfg)
		}
	}

	if err := config.Validate(base); err != nil {
		return config.EffectiveConfig{}, fmt.Errorf("validating config: %w", err)
	}

	return base, nil
}
