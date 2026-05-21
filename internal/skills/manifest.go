package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest tracks imported skills.
type Manifest struct {
	Version    int             `json:"version"`
	ImportedAt string          `json:"importedAt"`
	Skills     []ManifestEntry `json:"skills"`
}

// ManifestEntry describes a single imported skill.
type ManifestEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Target string `json:"target"`
	Scope  string `json:"scope"`
	SHA256 string `json:"sha256"`
}

// WriteManifest writes the manifest to disk.
func WriteManifest(path string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

// ReadManifest reads the manifest from disk.
func ReadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing manifest: %w", err)
	}
	return m, nil
}

func timeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
