package cli

import (
	"os"
	"path/filepath"
)

// RunPlan captures the parsed intent for a sandboxed OpenCode run.
type RunPlan struct {
	ProjectPath       string
	OpenCodeArgs      []string
	AllowHostAccess   bool
}

// parseRunArgs extracts the project path and remaining OpenCode arguments.
// Rules (from spec section 6.3):
//   - If the first positional arg is an existing directory, treat it as the
//     project path and forward the remaining args to OpenCode.
//   - If no project path is supplied, use the current working directory.
//   - If the first arg is not a wrapper subcommand and not an existing
//     directory, treat all args as OpenCode args.
//   - "--" is supported as an escape hatch but not required.
func parseRunArgs(args []string) (RunPlan, error) {
	var opencodeArgs []string
	var projectPath string

	// Handle "--" escape hatch: everything after -- is OpenCode args.
	var beforeEscape []string
	var afterEscape []string
	sawEscape := false
	for _, a := range args {
		if a == "--" && !sawEscape {
			sawEscape = true
			continue
		}
		if sawEscape {
			afterEscape = append(afterEscape, a)
		} else {
			beforeEscape = append(beforeEscape, a)
		}
	}

	// If we saw "--", the project path must be before it (if present),
	// and everything after is OpenCode args.
	if sawEscape {
		projectPath, opencodeArgs = extractProjectPath(beforeEscape)
		opencodeArgs = append(opencodeArgs, afterEscape...)
		return resolvePlan(projectPath, opencodeArgs)
	}

	// No escape hatch.
	projectPath, opencodeArgs = extractProjectPath(args)
	return resolvePlan(projectPath, opencodeArgs)
}

// extractProjectPath checks if the first positional arg is an existing
// directory. If so, it returns that directory and the remaining args.
func extractProjectPath(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	first := args[0]
	// Skip flag-like args when looking for project path.
	if len(first) > 0 && first[0] == '-' {
		return "", args
	}
	abs, err := filepath.Abs(first)
	if err != nil {
		return "", args
	}
	info, err := os.Stat(abs)
	if err == nil && info.IsDir() {
		return abs, args[1:]
	}
	return "", args
}

func resolvePlan(projectPath string, opencodeArgs []string) (RunPlan, error) {
	if projectPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return RunPlan{}, err
		}
		projectPath = wd
	}
	return RunPlan{
		ProjectPath:  projectPath,
		OpenCodeArgs: opencodeArgs,
	}, nil
}
