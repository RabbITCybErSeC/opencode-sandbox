package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const upgradeContainerfile = `ARG BASE_IMAGE
FROM ${BASE_IMAGE}
USER root
ARG OPENCODE_VERSION=latest
RUN npm install -g "opencode-ai@${OPENCODE_VERSION}" && npm cache clean --force
USER opencode
`

var runUpgradeCommand = func(cmd *exec.Cmd) error {
	return cmd.Run()
}

func runUpgrade(args []string) error {
	target, err := parseUpgradeTarget(args)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	cfg, err := loadRunConfig(wd, RunPlan{})
	if err != nil {
		return fmt.Errorf("loading runtime image config: %w", err)
	}
	image := cfg.Image.Name

	fmt.Printf("Pulling base runtime image %s...\n", image)
	if err := runUpgradeContainerCommand("image", "pull", image); err != nil {
		return fmt.Errorf("pulling runtime image %q: %w", image, err)
	}

	contextDir, err := os.MkdirTemp("", "opencode-sandbox-upgrade-*")
	if err != nil {
		return fmt.Errorf("creating temporary upgrade context: %w", err)
	}
	defer os.RemoveAll(contextDir)

	containerfile := filepath.Join(contextDir, "Containerfile")
	if err := os.WriteFile(containerfile, []byte(upgradeContainerfile), 0644); err != nil {
		return fmt.Errorf("writing temporary upgrade Containerfile: %w", err)
	}

	fmt.Printf("Building %s with OpenCode %s...\n", image, target)
	if err := runUpgradeContainerCommand(
		"build",
		"--file", containerfile,
		"--tag", image,
		"--build-arg", "BASE_IMAGE="+image,
		"--build-arg", "OPENCODE_VERSION="+target,
		contextDir,
	); err != nil {
		return fmt.Errorf("building upgraded runtime image %q: %w", image, err)
	}

	fmt.Printf("\nOpenCode %s is ready in %s.\n", target, image)
	return nil
}

func parseUpgradeTarget(args []string) (string, error) {
	if len(args) == 0 {
		return "latest", nil
	}
	if args[0] == "--method" || args[0] == "-m" {
		return "", fmt.Errorf("upgrade option %s is not supported: opencode-sandbox always upgrades with npm during image build", args[0])
	}
	if len(args) != 1 {
		return "", fmt.Errorf("usage: opencode-sandbox upgrade [target]")
	}
	if args[0] == "" || args[0][0] == '-' {
		return "", fmt.Errorf("unknown upgrade option: %s", args[0])
	}
	return args[0], nil
}

func runUpgradeContainerCommand(args ...string) error {
	cmd := exec.Command("container", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runUpgradeCommand(cmd)
}
