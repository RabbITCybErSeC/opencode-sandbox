package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveProjectPath validates and resolves the project path.
func ResolveProjectPath(path string) (string, error) {
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		path = wd
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("project path does not exist: %s", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", abs)
	}

	return abs, nil
}

// ValidateExtraMount checks that an extra mount is safe.
func ValidateExtraMount(mount string) error {
	parts := filepath.SplitList(mount)
	if len(parts) == 0 {
		return fmt.Errorf("empty mount")
	}
	// Basic validation: reject mounts that look like home or root.
	src := parts[0]
	if src == "/" || src == os.Getenv("HOME") {
		return fmt.Errorf("mount of %s is not allowed", src)
	}
	return nil
}
