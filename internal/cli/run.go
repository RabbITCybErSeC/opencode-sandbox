package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/containercmd"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/execx"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/runtime"
)

var runContainer = execx.RunContainer
var inspectRunImage = func(image string) ([]byte, error) {
	return exec.Command("container", "image", "inspect", image).CombinedOutput()
}

func runRun(args []string) error {
	dryRun := false
	printCommand := false
	keep := false
	debug := false
	allowHostAccess := false

	// Strip wrapper flags from args.
	var opencodeArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--print-command":
			printCommand = true
		case "--keep":
			keep = true
		case "--debug":
			debug = true
		case "--allow-host-access":
			allowHostAccess = true
		default:
			opencodeArgs = append(opencodeArgs, args[i])
		}
	}

	plan, err := parseRunArgs(opencodeArgs)
	if err != nil {
		return err
	}
	plan.AllowHostAccess = allowHostAccess

	containerPlan, cleanup, err := buildRunContainerPlan(plan, effectiveRunOptions{
		Keep:        keep,
		Debug:       debug,
		Materialize: !(dryRun || printCommand),
	})
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	argv := containercmd.BuildArgv(containerPlan)

	if dryRun {
		fmt.Println("Dry run - would execute:")
		fmt.Println(containercmd.RedactedCommand(argv))
		return nil
	}

	if printCommand {
		fmt.Println(containercmd.RedactedCommand(argv))
		return nil
	}

	if err := preflightContainerImages(containerPlan); err != nil {
		return err
	}

	if err := runContainer(argv); err != nil {
		return fmt.Errorf("running container: %w", err)
	}
	return nil
}

func preflightContainerImages(plan containercmd.Plan) error {
	if plan.Image != "" {
		if out, err := inspectRunImage(plan.Image); err != nil {
			return fmt.Errorf("required runtime image %q is not available locally: run `opencode-sandbox image pull` or `opencode-sandbox image build`; image inspect failed: %s", plan.Image, inspectOutput(out, err))
		}
	}

	if initImage := containercmd.RequiredInitImage(plan.Effective); initImage != "" {
		if out, err := inspectRunImage(initImage); err != nil {
			return fmt.Errorf("required init image %q is not available locally: run `opencode-sandbox image pull --strict-init` or disable command audit; image inspect failed: %s", initImage, inspectOutput(out, err))
		}
	}

	return nil
}

func inspectOutput(out []byte, err error) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		text = err.Error()
	}
	return text
}

type effectiveRunOptions struct {
	Keep        bool
	Debug       bool
	Materialize bool
}

func buildRunContainerPlan(plan RunPlan, opts effectiveRunOptions) (containercmd.Plan, func(), error) {
	projectPath, err := runtime.ResolveProjectPath(plan.ProjectPath)
	if err != nil {
		return containercmd.Plan{}, nil, err
	}

	effective, err := loadRunConfig(projectPath, plan)
	if err != nil {
		return containercmd.Plan{}, nil, err
	}

	runID := runtime.GenerateRunID()
	projectName := runtime.ProjectName(projectPath)
	stagingDir := "/tmp/opencode-sandbox-dry-run"
	eventLogDir := "/tmp/opencode-sandbox-dry-run/events"
	mergedSkillsDir := "/tmp/opencode-sandbox-dry-run/opencode/skills"
	var cleanup func()

	if opts.Materialize {
		stagingDir, err = runtime.StagingDir()
		if err != nil {
			return containercmd.Plan{}, nil, fmt.Errorf("creating staging dir: %w", err)
		}
		c := runtime.NewCleanup(opts.Keep)
		cleanup = func() { c.Remove(stagingDir) }

		if opts.Debug {
			fmt.Fprintf(os.Stderr, "staging dir: %s\n", stagingDir)
		}

		statePaths, err := runtime.EnsureOpenCodeState(effective)
		if err != nil {
			return containercmd.Plan{}, cleanup, fmt.Errorf("preparing opencode state: %w", err)
		}

		if err := runtime.GeneratePolicyBundle(stagingDir, runID, projectPath, projectName, effective); err != nil {
			return containercmd.Plan{}, cleanup, fmt.Errorf("generating policy bundle: %w", err)
		}

		eventLogBase := effective.Network.EBPF.EventLog
		if eventLogBase == "" {
			eventLogBase = effective.Audit.Commands.EventLog
		}
		eventLogDir, err = runtime.EventLogDirForBase(runID, eventLogBase)
		if err != nil {
			return containercmd.Plan{}, cleanup, fmt.Errorf("resolving event log dir: %w", err)
		}
		if err := os.MkdirAll(eventLogDir, 0755); err != nil {
			return containercmd.Plan{}, cleanup, fmt.Errorf("creating event log dir: %w", err)
		}

		mergedSkillsDir, err = runtime.MergeSkills(stagingDir, projectPath, effective.Skills)
		if err != nil {
			return containercmd.Plan{}, cleanup, fmt.Errorf("merging skills: %w", err)
		}

		return containercmd.Plan{
			ProjectPath:       projectPath,
			StagingDir:        stagingDir,
			MergedSkillsDir:   mergedSkillsDir,
			EventLogDir:       eventLogDir,
			OpenCodeConfigDir: statePaths.ConfigDir,
			OpenCodeDataDir:   statePaths.DataDir,
			OpenCodeStateDir:  statePaths.StateDir,
			Image:             effective.Image.Name,
			OpenCodeArgs:      plan.OpenCodeArgs,
			Effective:         effective,
		}, cleanup, nil
	}

	statePaths, err := runtime.ResolveOpenCodeStatePaths()
	if err != nil {
		return containercmd.Plan{}, nil, fmt.Errorf("resolving opencode state paths: %w", err)
	}

	return containercmd.Plan{
		ProjectPath:       projectPath,
		StagingDir:        stagingDir,
		MergedSkillsDir:   mergedSkillsDir,
		EventLogDir:       eventLogDir,
		OpenCodeConfigDir: statePaths.ConfigDir,
		OpenCodeDataDir:   statePaths.DataDir,
		OpenCodeStateDir:  statePaths.StateDir,
		Image:             effective.Image.Name,
		OpenCodeArgs:      plan.OpenCodeArgs,
		Effective:         effective,
	}, nil, nil
}

func loadRunConfig(projectPath string, plan RunPlan) (config.EffectiveConfig, error) {
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

	// CLI flag override: --allow-host-access
	if plan.AllowHostAccess {
		base.Network.LocalhostAccess.Enabled = true
	}

	if err := config.Validate(base); err != nil {
		return config.EffectiveConfig{}, fmt.Errorf("validating config: %w", err)
	}

	return base, nil
}
