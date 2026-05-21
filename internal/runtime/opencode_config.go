package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

// OverlayConfig describes the generated opencode.json overlay.
type OverlayConfig struct {
	Autoupdate bool `json:"autoupdate"`
}

// OpenCodeStatePaths are host-side durable directories mounted into the
// sandboxed OpenCode home.
type OpenCodeStatePaths struct {
	ConfigDir string
	DataDir   string
	StateDir  string
}

// ResolveOpenCodeStatePaths resolves wrapper-owned durable OpenCode paths.
func ResolveOpenCodeStatePaths() (OpenCodeStatePaths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return OpenCodeStatePaths{}, fmt.Errorf("getting user config dir: %w", err)
	}
	base := filepath.Join(configRoot, "opencode-sandbox", "opencode")
	return OpenCodeStatePaths{
		ConfigDir: filepath.Join(base, "config"),
		DataDir:   filepath.Join(base, "data"),
		StateDir:  filepath.Join(base, "state"),
	}, nil
}

// EnsureOpenCodeState creates durable host-side OpenCode state and writes the
// generated overlay when enabled.
func EnsureOpenCodeState(cfg config.EffectiveConfig) (OpenCodeStatePaths, error) {
	paths, err := ResolveOpenCodeStatePaths()
	if err != nil {
		return OpenCodeStatePaths{}, err
	}
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.StateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return OpenCodeStatePaths{}, fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	if cfg.OpenCode.MountHostConfig {
		if err := copyHostOpenCodeConfig(paths.ConfigDir); err != nil {
			return OpenCodeStatePaths{}, err
		}
	}
	if cfg.OpenCode.GeneratedConfig {
		if err := WriteOpenCodeOverlay(paths.ConfigDir, cfg.OpenCode.Autoupdate); err != nil {
			return OpenCodeStatePaths{}, err
		}
	}
	return paths, nil
}

// GenerateOpenCodeConfig writes the overlay opencode.json into sandboxHome.
func GenerateOpenCodeConfig(sandboxHome string) error {
	configDir := filepath.Join(sandboxHome, ".config", "opencode")
	return WriteOpenCodeOverlay(configDir, false)
}

// WriteOpenCodeOverlay writes opencode.json into an OpenCode config directory.
func WriteOpenCodeOverlay(configDir string, autoupdate bool) error {
	overlay := OverlayConfig{
		Autoupdate: autoupdate,
	}
	data, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling overlay config: %w", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	path := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing overlay config: %w", err)
	}

	return nil
}

func copyHostOpenCodeConfig(destConfigDir string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("getting user config dir: %w", err)
	}
	return copySelectedOpenCodeConfig(filepath.Join(configDir, "opencode"), destConfigDir)
}

// StageOpenCodeConfig copies host OpenCode config into the sandbox home.
// It copies tui.json and any other non-overlay files, then generates
// the overlay opencode.json.
func StageOpenCodeConfig(sandboxHome string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("getting user config dir: %w", err)
	}
	hostOpenCodeDir := filepath.Join(configDir, "opencode")
	return StageOpenCodeConfigFrom(sandboxHome, hostOpenCodeDir)
}

// StageOpenCodeConfigFrom copies host OpenCode config from a specific source
// directory into the sandbox home.
func StageOpenCodeConfigFrom(sandboxHome, hostOpenCodeDir string) error {
	// Ensure sandbox opencode config dir exists.
	sandboxConfigDir := filepath.Join(sandboxHome, ".config", "opencode")
	if err := copySelectedOpenCodeConfig(hostOpenCodeDir, sandboxConfigDir); err != nil {
		return err
	}
	return GenerateOpenCodeConfig(sandboxHome)
}

func copySelectedOpenCodeConfig(hostOpenCodeDir, sandboxConfigDir string) error {
	if err := os.MkdirAll(sandboxConfigDir, 0755); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	if err := copyIfExists(filepath.Join(hostOpenCodeDir, "tui.json"), filepath.Join(sandboxConfigDir, "tui.json")); err != nil {
		return err
	}
	// Copy auth/session files if known.
	// TODO: verify exact auth file names from OpenCode source/docs.
	knownFiles := []string{
		"session.json",
		"auth.json",
		"credentials.json",
	}
	for _, name := range knownFiles {
		if err := copyIfExists(
			filepath.Join(hostOpenCodeDir, name),
			filepath.Join(sandboxConfigDir, name),
		); err != nil {
			return err
		}
	}
	return nil
}

func copyIfExists(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	return nil
}
