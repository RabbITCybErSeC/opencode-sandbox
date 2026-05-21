package cli

import (
	"fmt"
	"os"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/doctor"
)

func runDoctor(args []string) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	// Load global config for eBPF checks.
	cfg := config.Defaults()
	globalPath, err := config.GlobalConfigPath()
	if err == nil {
		if _, err := os.Stat(globalPath); err == nil {
			globalCfg, err := config.Load(globalPath)
			if err == nil {
				cfg = config.MergeEffective(cfg, globalCfg)
			}
		}
	}

	checks := doctor.Run(cfg)
	doctor.PrintReport(checks, jsonOut)

	if !doctor.IsHealthy(checks) {
		return fmt.Errorf("doctor found failures")
	}
	return nil
}
