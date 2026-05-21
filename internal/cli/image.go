package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

var installedSourceDir string

var runImageCommand = func(cmd *exec.Cmd) error {
	return cmd.Run()
}

func runImage(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: opencode-sandbox image <pull|build> [options]")
	}

	subcmd := args[0]
	switch subcmd {
	case "build":
		return runImageBuild(args[1:])
	case "pull":
		return runImagePull(args[1:])
	default:
		return fmt.Errorf("unknown image subcommand: %s", subcmd)
	}
}

func runImageBuild(args []string) error {
	strictInit := false
	tagSet := false
	tag := defaultImageTag(strictInit)
	opencodeVersion := ""
	contextDir := ""
	containerfile := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tag":
			if i+1 >= len(args) {
				return fmt.Errorf("--tag requires a value")
			}
			tag = args[i+1]
			tagSet = true
			i++
		case "--opencode-version":
			if i+1 >= len(args) {
				return fmt.Errorf("--opencode-version requires a value")
			}
			opencodeVersion = args[i+1]
			i++
		case "--strict-init":
			strictInit = true
			if !tagSet {
				tag = defaultImageTag(strictInit)
			}
		case "--context":
			if i+1 >= len(args) {
				return fmt.Errorf("--context requires a value")
			}
			contextDir = args[i+1]
			i++
		case "--file":
			if i+1 >= len(args) {
				return fmt.Errorf("--file requires a value")
			}
			containerfile = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown image build option: %s", args[i])
		}
	}

	source, err := resolveBuildSource(strictInit, containerfile, contextDir)
	if err != nil {
		name := "Containerfile"
		if strictInit {
			name = "Containerfile.init"
		}
		return fmt.Errorf("%s not found: %w", name, err)
	}

	buildArgs := []string{
		"build",
		"--file", source.Containerfile,
		"--tag", tag,
	}

	if opencodeVersion != "" {
		buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("OPENCODE_VERSION=%s", opencodeVersion))
	}

	buildArgs = append(buildArgs, source.ContextDir)

	fmt.Printf("Building image %s...\n", tag)
	fmt.Printf("  Containerfile: %s\n", source.Containerfile)
	fmt.Printf("  Context: %s\n", source.ContextDir)
	if opencodeVersion != "" {
		fmt.Printf("  OpenCode version: %s\n", opencodeVersion)
	}
	fmt.Println()

	cmd := exec.Command("container", buildArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := runImageCommand(cmd); err != nil {
		return fmt.Errorf("container build failed: %w", err)
	}

	fmt.Printf("\nImage %s built successfully.\n", tag)
	fmt.Printf("Next: opencode-sandbox run .\n")
	return nil
}

func runImagePull(args []string) error {
	strictInit := false
	tagSet := false
	tag := defaultImageTag(strictInit)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--strict-init":
			strictInit = true
			if !tagSet {
				tag = defaultImageTag(strictInit)
			}
		case "--tag":
			if i+1 >= len(args) {
				return fmt.Errorf("--tag requires a value")
			}
			tag = args[i+1]
			tagSet = true
			i++
		default:
			return fmt.Errorf("unknown image pull option: %s", args[i])
		}
	}

	fmt.Printf("Pulling image %s...\n", tag)

	cmd := exec.Command("container", "image", "pull", tag)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := runImageCommand(cmd); err != nil {
		return fmt.Errorf("container image pull failed: %w", err)
	}

	fmt.Printf("\nImage %s pulled successfully.\n", tag)
	return nil
}

func defaultImageTag(strictInit bool) string {
	if strictInit {
		return config.DefaultStrictInitImage
	}
	return config.DefaultImageName
}

type buildSource struct {
	Containerfile string
	ContextDir    string
}

func resolveBuildSource(strictInit bool, explicitFile, explicitContext string) (buildSource, error) {
	if explicitFile != "" {
		containerfile, err := absExistingFile(explicitFile)
		if err != nil {
			return buildSource{}, err
		}
		contextDir := filepath.Dir(containerfile)
		if explicitContext != "" {
			contextDir, err = absExistingDir(explicitContext)
			if err != nil {
				return buildSource{}, err
			}
		}
		return buildSource{Containerfile: containerfile, ContextDir: contextDir}, nil
	}

	if explicitContext != "" {
		return findBuildSourceInDir(strictInit, explicitContext)
	}

	for _, dir := range []string{os.Getenv("OPENCODE_SANDBOX_DIR"), installedSourceDir} {
		if dir == "" {
			continue
		}
		if source, err := findBuildSourceInDir(strictInit, dir); err == nil {
			return source, nil
		}
	}

	if containerfile := findContainerfileFromCurrentTree(strictInit); containerfile != "" {
		return buildSource{Containerfile: containerfile, ContextDir: filepath.Dir(containerfile)}, nil
	}

	return buildSource{}, fmt.Errorf("searched --context, --file, OPENCODE_SANDBOX_DIR, installed source dir, current directory, and parents")
}

func findContainerfile(strictInit bool) string {
	source, err := resolveBuildSource(strictInit, "", "")
	if err != nil {
		return ""
	}
	return source.Containerfile
}

func findBuildSourceInDir(strictInit bool, dir string) (buildSource, error) {
	root, err := absExistingDir(dir)
	if err != nil {
		return buildSource{}, err
	}

	if containerfile := findContainerfileInDir(strictInit, root); containerfile != "" {
		return buildSource{Containerfile: containerfile, ContextDir: root}, nil
	}

	name := "Containerfile"
	if strictInit {
		name = "Containerfile.init"
	}
	return buildSource{}, fmt.Errorf("%s not found in %s", name, root)
}

func findContainerfileInDir(strictInit bool, dir string) string {
	suffix := ""
	if strictInit {
		suffix = ".init"
	}

	for _, name := range []string{"Containerfile" + suffix, "Dockerfile" + suffix} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findContainerfileFromCurrentTree(strictInit bool) string {
	suffix := ""
	if strictInit {
		suffix = ".init"
	}

	// Check current directory first.
	for _, name := range []string{"Containerfile" + suffix, "Dockerfile" + suffix} {
		if _, err := os.Stat(name); err == nil {
			abs, _ := filepath.Abs(name)
			return abs
		}
	}

	// Check repo root (look upward for .git).
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		for _, name := range []string{"Containerfile" + suffix, "Dockerfile" + suffix} {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

func absExistingFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", abs)
	}
	return abs, nil
}

func absExistingDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}
