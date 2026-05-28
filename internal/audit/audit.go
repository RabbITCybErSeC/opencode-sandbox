package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	SchemaVersion = 1

	DefaultFileName         = "audit-events.jsonl"
	DefaultRotationMaxBytes = int64(10 * 1024 * 1024)
	DefaultRotationMaxFiles = 5

	EventCommandExec    = "command.exec"
	EventNetworkConnect = "network.connect"
	EventDaemonHealth   = "daemon.health"
	EventLogRotate      = "log.rotate"
	EventAuditError     = "audit.error"
)

// RotationConfig controls size-based JSONL rotation.
type RotationConfig struct {
	MaxBytes int64 `json:"maxBytes" yaml:"maxBytes,omitempty"`
	MaxFiles int   `json:"maxFiles" yaml:"maxFiles,omitempty"`
}

// Event is the unified audit record written to audit-events.jsonl.
type Event struct {
	SchemaVersion int    `json:"schemaVersion"`
	EventType     string `json:"eventType"`
	TS            string `json:"ts"`
	RunID         string `json:"runId,omitempty"`
	Project       string `json:"project,omitempty"`
	Backend       string `json:"backend,omitempty"`

	Hook      string `json:"hook,omitempty"`
	Component string `json:"component,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	Active    *bool  `json:"active,omitempty"`
	Attached  *bool  `json:"attached,omitempty"`

	PID       int      `json:"pid,omitempty"`
	PPID      int      `json:"ppid,omitempty"`
	UID       int      `json:"uid,omitempty"`
	GID       int      `json:"gid,omitempty"`
	Process   string   `json:"process,omitempty"`
	CWD       string   `json:"cwd,omitempty"`
	Exe       string   `json:"exe,omitempty"`
	Argv      []string `json:"argv,omitempty"`
	Argc      int      `json:"argc,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`

	Protocol    string `json:"protocol,omitempty"`
	Host        string `json:"host,omitempty"`
	DstIP       string `json:"dstIp,omitempty"`
	DstPort     int    `json:"dstPort,omitempty"`
	Method      string `json:"method,omitempty"`
	Decision    string `json:"decision,omitempty"`
	Reason      string `json:"reason,omitempty"`
	MatchedRule string `json:"matchedRule,omitempty"`

	LostSamples  uint64 `json:"lostSamples,omitempty"`
	PreviousPath string `json:"previousPath,omitempty"`
	NextPath     string `json:"nextPath,omitempty"`
}

// Writer appends structured audit events to a host log and optional mirror.
type Writer struct {
	mu      sync.Mutex
	host    *sink
	mirror  *sink
	rotator RotationConfig
}

type sink struct {
	path string
	file *os.File
	size int64
}

// NewWriter opens the configured audit sinks.
func NewWriter(hostPath, mirrorPath string, mirror bool, rotation RotationConfig) (*Writer, error) {
	if hostPath == "" {
		return nil, fmt.Errorf("audit host path is empty")
	}
	if rotation.MaxBytes < 0 {
		return nil, fmt.Errorf("audit rotation maxBytes must be non-negative")
	}
	if rotation.MaxFiles < 0 {
		return nil, fmt.Errorf("audit rotation maxFiles must be non-negative")
	}
	host, err := openSink(hostPath)
	if err != nil {
		return nil, err
	}
	var mirrorSink *sink
	if mirror && mirrorPath != "" {
		mirrorSink, err = openSink(mirrorPath)
		if err != nil {
			_ = host.file.Close()
			return nil, err
		}
	}
	return &Writer{host: host, mirror: mirrorSink, rotator: rotation}, nil
}

func openSink(path string) (*sink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating audit log dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening audit log file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat audit log file: %w", err)
	}
	return &sink{path: path, file: file, size: info.Size()}, nil
}

// Write appends one audit event to all configured sinks.
func (w *Writer) Write(ev Event) error {
	if ev.SchemaVersion == 0 {
		ev.SchemaVersion = SchemaVersion
	}
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := marshalLine(ev)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeededLocked(int64(len(line))); err != nil {
		return err
	}
	return w.writeLineLocked(line)
}

func marshalLine(ev Event) ([]byte, error) {
	line, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshaling audit event: %w", err)
	}
	return append(line, '\n'), nil
}

func (w *Writer) rotateIfNeededLocked(incoming int64) error {
	if w.rotator.MaxBytes <= 0 || w.host.size == 0 || w.host.size+incoming <= w.rotator.MaxBytes {
		return nil
	}
	previous := w.host.path
	if err := w.rotateSinkLocked(w.host); err != nil {
		return err
	}
	if w.mirror != nil {
		if err := w.rotateSinkLocked(w.mirror); err != nil {
			return err
		}
	}
	rotationEvent := Event{
		SchemaVersion: SchemaVersion,
		EventType:     EventLogRotate,
		TS:            time.Now().UTC().Format(time.RFC3339Nano),
		PreviousPath:  previous,
		NextPath:      w.host.path,
	}
	line, err := marshalLine(rotationEvent)
	if err != nil {
		return err
	}
	return w.writeLineLocked(line)
}

func (w *Writer) rotateSinkLocked(s *sink) error {
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("closing audit log for rotation: %w", err)
	}
	if w.rotator.MaxFiles <= 0 {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing audit log during rotation: %w", err)
		}
	} else {
		oldest := rotatedPath(s.path, w.rotator.MaxFiles)
		if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing old rotated audit log: %w", err)
		}
		for i := w.rotator.MaxFiles - 1; i >= 1; i-- {
			from := rotatedPath(s.path, i)
			to := rotatedPath(s.path, i+1)
			if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("renaming rotated audit log: %w", err)
			}
		}
		if err := os.Rename(s.path, rotatedPath(s.path, 1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotating audit log: %w", err)
		}
	}
	reopened, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("reopening audit log after rotation: %w", err)
	}
	s.file = reopened
	s.size = 0
	return nil
}

func rotatedPath(path string, index int) string {
	return fmt.Sprintf("%s.%d", path, index)
}

func (w *Writer) writeLineLocked(line []byte) error {
	if err := writeSinkLine(w.host, line); err != nil {
		return err
	}
	if w.mirror != nil {
		if err := writeSinkLine(w.mirror, line); err != nil {
			return err
		}
	}
	return nil
}

func writeSinkLine(s *sink, line []byte) error {
	if _, err := s.file.Write(line); err != nil {
		return fmt.Errorf("writing audit event: %w", err)
	}
	s.size += int64(len(line))
	return nil
}

// Close closes all configured audit sinks.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	if w.host != nil && w.host.file != nil {
		if err := w.host.file.Close(); err != nil {
			firstErr = err
		}
	}
	if w.mirror != nil && w.mirror.file != nil {
		if err := w.mirror.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
