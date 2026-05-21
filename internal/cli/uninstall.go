package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/runtime"
)

var runUninstallCommand = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.CombinedOutput()
}

var uninstallCleanupTimeout = 20 * time.Second

type uninstallOptions struct {
	DryRun bool
}

type containerListEntry struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Names []string `json:"names"`
}

func runUninstall(args []string) error {
	opts := uninstallOptions{}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		default:
			return fmt.Errorf("unknown uninstall option: %s", arg)
		}
	}

	effective := config.Defaults()
	if globalPath, err := config.GlobalConfigPath(); err == nil {
		if _, err := os.Stat(globalPath); err == nil {
			if globalCfg, err := config.Load(globalPath); err == nil {
				effective = config.MergeEffective(effective, globalCfg)
			}
		}
	}

	paths, err := uninstallPaths()
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Println("Dry run - would uninstall opencode-sandbox global artifacts:")
	} else {
		fmt.Println("Uninstalling opencode-sandbox global artifacts:")
	}

	for _, path := range paths {
		if err := removePath(path, opts.DryRun); err != nil {
			return err
		}
	}
	if err := removeTempStagingDirs(opts.DryRun); err != nil {
		return err
	}

	if err := removeContainerArtifacts(effective, opts.DryRun); err != nil {
		return err
	}

	fmt.Println("Project-local artifacts are intentionally preserved: .opencode-sandbox.yaml and .opencode-sandbox/.")
	return nil
}

func uninstallPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("getting config directory: %w", err)
	}

	binPath := os.Getenv("OPENCODE_SANDBOX_BIN")
	if binPath == "" {
		binPath = filepath.Join(home, ".local", "bin", "opencode-sandbox")
	}
	srcDir := os.Getenv("OPENCODE_SANDBOX_DIR")
	if srcDir == "" {
		srcDir = filepath.Join(home, ".local", "share", "opencode-sandbox-src")
	}

	return []string{
		binPath,
		srcDir,
		filepath.Join(configRoot, "opencode-sandbox"),
		filepath.Join(home, ".local", "state", "opencode-sandbox"),
	}, nil
}

func removePath(path string, dryRun bool) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("  skip missing %s\n", path)
		return nil
	} else if err != nil {
		return fmt.Errorf("checking %s: %w", path, err)
	}

	if dryRun {
		fmt.Printf("  would remove %s\n", path)
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	fmt.Printf("  removed %s\n", path)
	return nil
}

func removeTempStagingDirs(dryRun bool) error {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "opencode-sandbox-*"))
	if err != nil {
		return fmt.Errorf("finding temp staging dirs: %w", err)
	}
	for _, path := range matches {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking %s: %w", path, err)
		}
		if !info.IsDir() {
			continue
		}
		if dryRun {
			fmt.Printf("  would remove %s\n", path)
			continue
		}
		if err := runtime.SafeRemoveAll(path); err != nil {
			return err
		}
		fmt.Printf("  removed %s\n", path)
	}
	return nil
}

func removeContainerArtifacts(effective config.EffectiveConfig, dryRun bool) error {
	if dryRun {
		fmt.Printf("  would remove Apple container resources with prefix %q\n", effective.Container.NamePrefix)
		for _, image := range uninstallImages(effective) {
			fmt.Printf("  would remove image %s\n", image)
		}
		fmt.Println("  would delete Apple container builder")
		return nil
	}

	if err := removeContainersWithPrefix(effective.Container.NamePrefix); err != nil {
		return err
	}
	for _, image := range uninstallImages(effective) {
		if err := runBestEffort("removing image "+image, "container", "image", "delete", "--force", image); err != nil {
			return err
		}
	}
	if err := runBestEffort("deleting Apple container builder", "container", "builder", "delete", "--force"); err != nil {
		return err
	}
	return nil
}

func removeContainersWithPrefix(prefix string) error {
	out, err := runUninstallCommand(exec.Command("container", "list", "--all", "--format", "json"))
	if err != nil {
		if commandNotFound(err) {
			fmt.Println("  skip Apple container resources: container command not found")
			return nil
		}
		return fmt.Errorf("listing containers: %w", err)
	}

	var entries []containerListEntry
	if len(strings.TrimSpace(string(out))) > 0 {
		if err := json.Unmarshal(out, &entries); err != nil {
			return fmt.Errorf("parsing container list output: %w", err)
		}
	}

	for _, entry := range entries {
		if !containerEntryMatchesPrefix(entry, prefix) {
			continue
		}
		id := entry.ID
		if id == "" {
			id = entry.Name
		}
		if id == "" {
			continue
		}
		if err := runBestEffort("removing container "+id, "container", "delete", "--force", id); err != nil {
			return err
		}
	}
	return nil
}

func containerEntryMatchesPrefix(entry containerListEntry, prefix string) bool {
	if strings.HasPrefix(entry.Name, prefix) {
		return true
	}
	for _, name := range entry.Names {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func uninstallImages(effective config.EffectiveConfig) []string {
	seen := map[string]bool{}
	var images []string
	for _, image := range []string{
		effective.Image.Name,
		effective.Image.StrictInitImage,
		effective.Network.EBPF.InitImage,
		config.DefaultImageName,
		config.DefaultStrictInitImage,
	} {
		if image == "" || seen[image] {
			continue
		}
		seen[image] = true
		images = append(images, image)
	}
	return images
}

func runBestEffort(label string, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uninstallCleanupTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	out, err := runUninstallCommand(cmd)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("  skip %s: timed out; continuing\n", label)
			return nil
		}
		if commandNotFound(err) {
			fmt.Printf("  skip %s: %s command not found\n", label, name)
			return nil
		}
		if looksMissing(out) {
			fmt.Printf("  skip %s: already absent\n", label)
			return nil
		}
		return fmt.Errorf("%s: %w", label, err)
	}
	fmt.Printf("  %s\n", label)
	return nil
}

func commandNotFound(err error) bool {
	if execErr, ok := err.(*exec.Error); ok {
		return execErr.Err == exec.ErrNotFound
	}
	return false
}

func looksMissing(out []byte) bool {
	text := strings.ToLower(string(out))
	return strings.Contains(text, "not found") ||
		strings.Contains(text, "no such") ||
		strings.Contains(text, "does not exist") ||
		strings.Contains(text, "unknown image")
}
