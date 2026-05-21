package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"
)

// ContainerName generates a container name with the given prefix.
func ContainerName(prefix string) string {
	return fmt.Sprintf("%s-run", prefix)
}

// GenerateRunID creates a unique run identifier.
func GenerateRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp-based ID if crypto rand fails.
		return fmt.Sprintf("opencode-sandbox-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("opencode-sandbox-%s", hex.EncodeToString(b))
}

// ProjectName extracts the project name from a path.
func ProjectName(projectPath string) string {
	return filepath.Base(projectPath)
}
