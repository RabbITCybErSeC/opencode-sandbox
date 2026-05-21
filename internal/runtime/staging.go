package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StagingDir creates a per-run staging directory with the expected layout.
func StagingDir() (string, error) {
	dir, err := os.MkdirTemp("", "opencode-sandbox-*")
	if err != nil {
		return "", fmt.Errorf("creating staging dir: %w", err)
	}
	for _, sub := range []string{"home", "opencode", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("creating staging subdir %s: %w", sub, err)
		}
	}
	return dir, nil
}

// SafeRemoveAll deletes dir only if it looks like a wrapper-created temp dir.
func SafeRemoveAll(dir string) error {
	if !isWrapperTempDir(dir) {
		return fmt.Errorf("refusing to remove non-wrapper directory: %s", dir)
	}
	return os.RemoveAll(dir)
}

func isWrapperTempDir(dir string) bool {
	base := filepath.Base(dir)
	return strings.HasPrefix(base, "opencode-sandbox-")
}
