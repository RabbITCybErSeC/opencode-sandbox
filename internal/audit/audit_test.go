package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterEmitsJSONLWithCommonFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFileName)
	w, err := NewWriter(path, "", false, RotationConfig{MaxBytes: DefaultRotationMaxBytes, MaxFiles: DefaultRotationMaxFiles})
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer w.Close()

	if err := w.Write(Event{EventType: EventDaemonHealth, RunID: "run-1", Project: "proj", Backend: "ebpf", Status: "ready"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL line, got %d", len(lines))
	}
	var parsed Event
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersion || parsed.EventType != EventDaemonHealth {
		t.Fatalf("unexpected common fields: %+v", parsed)
	}
	if parsed.TS == "" {
		t.Fatal("expected timestamp")
	}
}

func TestWriterMirrorsEvents(t *testing.T) {
	dir := t.TempDir()
	host := filepath.Join(dir, "host", DefaultFileName)
	mirror := filepath.Join(dir, "mirror", DefaultFileName)
	w, err := NewWriter(host, mirror, true, RotationConfig{MaxBytes: DefaultRotationMaxBytes, MaxFiles: DefaultRotationMaxFiles})
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	if err := w.Write(Event{EventType: EventCommandExec, RunID: "run-1", Exe: "/bin/sh"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	hostData, _ := os.ReadFile(host)
	mirrorData, _ := os.ReadFile(mirror)
	if string(hostData) != string(mirrorData) {
		t.Fatalf("expected mirror to match host\nhost=%s\nmirror=%s", hostData, mirrorData)
	}
}

func TestWriterRotatesBySizeAndKeepsRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFileName)
	w, err := NewWriter(path, "", false, RotationConfig{MaxBytes: 180, MaxFiles: 2})
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	for i := 0; i < 6; i++ {
		if err := w.Write(Event{EventType: EventAuditError, RunID: "run-1", Error: strings.Repeat("x", 40)}); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected active log: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected first rotated log: %v", err)
	}
	if _, err := os.Stat(path + ".2"); err != nil {
		t.Fatalf("expected second rotated log: %v", err)
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("expected retention to prune third rotated log")
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), EventLogRotate) {
		t.Fatalf("expected active log to contain rotate event, got %s", data)
	}
}
