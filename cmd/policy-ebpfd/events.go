package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event mirrors internal/runtime.NetworkEvent for the daemon.
type Event struct {
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

// CommandEvent is a single command execution audit record.
type CommandEvent struct {
	TS        string   `json:"ts"`
	RunID     string   `json:"runId"`
	Project   string   `json:"project"`
	Backend   string   `json:"backend"`
	Hook      string   `json:"hook"`
	PID       int      `json:"pid"`
	PPID      int      `json:"ppid"`
	UID       int      `json:"uid"`
	GID       int      `json:"gid"`
	CWD       string   `json:"cwd,omitempty"`
	Exe       string   `json:"exe,omitempty"`
	Argv      []string `json:"argv,omitempty"`
	Argc      int      `json:"argc"`
	Truncated bool     `json:"truncated"`
	Decision  string   `json:"decision"`
	Reason    string   `json:"reason"`
}

// DaemonEventWriter writes JSONL events from inside the init container.
type DaemonEventWriter struct {
	mu         sync.Mutex
	hostFile   *os.File
	mirrorFile *os.File
}

// NewDaemonEventWriter opens the event sinks configured in the bundle.
func NewDaemonEventWriter(hostPath, mirrorPath string, mirror bool) (*DaemonEventWriter, error) {
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return nil, fmt.Errorf("creating host event dir: %w", err)
	}
	hostFile, err := os.OpenFile(hostPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening host event file: %w", err)
	}

	var mirrorFile *os.File
	if mirror && mirrorPath != "" {
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

	return &DaemonEventWriter{
		hostFile:   hostFile,
		mirrorFile: mirrorFile,
	}, nil
}

// Write emits a single event to all configured sinks.
func (w *DaemonEventWriter) Write(ev Event) error {
	return w.writeJSON(ev)
}

// WriteCommand emits a single command audit event to all configured sinks.
func (w *DaemonEventWriter) WriteCommand(ev CommandEvent) error {
	return w.writeJSON(ev)
}

func (w *DaemonEventWriter) writeJSON(ev any) error {
	switch typed := ev.(type) {
	case Event:
		if typed.TS == "" {
			typed.TS = time.Now().UTC().Format(time.RFC3339)
		}
		ev = typed
	case CommandEvent:
		if typed.TS == "" {
			typed.TS = time.Now().UTC().Format(time.RFC3339)
		}
		ev = typed
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

// Close flushes and closes all sinks.
func (w *DaemonEventWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	if err := w.hostFile.Close(); err != nil {
		firstErr = err
	}
	if w.mirrorFile != nil {
		if err := w.mirrorFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
