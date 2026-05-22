package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NetworkEvent is a single eBPF network monitoring record.
type NetworkEvent struct {
	TS          string `json:"ts"`
	RunID       string `json:"runId"`
	Project     string `json:"project"`
	Backend     string `json:"backend"`
	Hook        string `json:"hook"`
	PID         int    `json:"pid,omitempty"`
	Process     string `json:"process,omitempty"`
	Protocol    string `json:"protocol"`
	DstIP       string `json:"dstIp"`
	DstPort     int    `json:"dstPort"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
	MatchedRule string `json:"matchedRule,omitempty"`
}

// EventWriter writes JSONL network events durably.
type EventWriter struct {
	mu         sync.Mutex
	hostFile   *os.File
	mirrorFile *os.File
}

// NewEventWriter creates an event writer for the given paths.
// hostPath is always written. mirrorPath is written only if non-empty.
func NewEventWriter(hostPath, mirrorPath string) (*EventWriter, error) {
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return nil, fmt.Errorf("creating host event dir: %w", err)
	}
	hostFile, err := os.OpenFile(hostPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening host event file: %w", err)
	}

	var mirrorFile *os.File
	if mirrorPath != "" {
		if err := os.MkdirAll(filepath.Dir(mirrorPath), 0755); err != nil {
			hostFile.Close()
			return nil, fmt.Errorf("creating mirror event dir: %w", err)
		}
		mirrorFile, err = os.OpenFile(mirrorPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			hostFile.Close()
			return nil, fmt.Errorf("opening mirror event file: %w", err)
		}
	}

	return &EventWriter{
		hostFile:   hostFile,
		mirrorFile: mirrorFile,
	}, nil
}

// WriteEvent emits a single event to all configured sinks.
func (w *EventWriter) WriteEvent(ev NetworkEvent) error {
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339)
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.hostFile.Write(line); err != nil {
		return fmt.Errorf("writing host event: %w", err)
	}
	if w.mirrorFile != nil {
		if _, err := w.mirrorFile.Write(line); err != nil {
			return fmt.Errorf("writing mirror event: %w", err)
		}
	}
	return nil
}

// Close flushes and closes all event sinks.
func (w *EventWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var errs []error
	if err := w.hostFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if w.mirrorFile != nil {
		if err := w.mirrorFile.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// EventLogDir returns the durable host event directory for a run.
func EventLogDir(runID string) (string, error) {
	return EventLogDirForBase(runID, "")
}

// EventLogDirForBase returns the durable host event directory for a run,
// optionally rooted at a configured base directory.
func EventLogDirForBase(runID, baseDir string) (string, error) {
	root, err := EventLogBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, runID), nil
}

// EventLogBaseDir returns the host directory containing per-run log dirs.
func EventLogBaseDir(baseDir string) (string, error) {
	if baseDir != "" {
		return expandHome(baseDir), nil
	}
	stateDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, ".local", "state", "opencode-sandbox", "runs"), nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
