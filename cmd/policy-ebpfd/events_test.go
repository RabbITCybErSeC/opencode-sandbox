package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
)

func testRotation() auditlog.RotationConfig {
	return auditlog.RotationConfig{MaxBytes: auditlog.DefaultRotationMaxBytes, MaxFiles: auditlog.DefaultRotationMaxFiles}
}

func TestDaemonEventWriterCreatesHostFile(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewDaemonEventWriter(hostPath, "", false, testRotation())
	if err != nil {
		t.Fatalf("NewDaemonEventWriter failed: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(hostPath); err != nil {
		t.Errorf("expected host file to exist: %v", err)
	}
}

func TestDaemonEventWriterCreatesMirrorWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "host.jsonl")
	mirrorPath := filepath.Join(dir, "mirror.jsonl")

	w, err := NewDaemonEventWriter(hostPath, mirrorPath, true, testRotation())
	if err != nil {
		t.Fatalf("NewDaemonEventWriter failed: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(mirrorPath); err != nil {
		t.Errorf("expected mirror file to exist: %v", err)
	}
}

func TestDaemonEventWriterOmitsMirrorWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "host.jsonl")
	mirrorPath := filepath.Join(dir, "mirror.jsonl")

	w, err := NewDaemonEventWriter(hostPath, mirrorPath, false, testRotation())
	if err != nil {
		t.Fatalf("NewDaemonEventWriter failed: %v", err)
	}
	defer w.Close()

	// Mirror file should not be created when disabled.
	if _, err := os.Stat(mirrorPath); !os.IsNotExist(err) {
		t.Error("expected mirror file to not exist when mirror=false")
	}
}

func TestDaemonEventWriterEmitsValidJSONL(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewDaemonEventWriter(hostPath, "", false, testRotation())
	if err != nil {
		t.Fatalf("NewDaemonEventWriter failed: %v", err)
	}

	ev := Event{
		RunID:       "run-1",
		Project:     "myproj",
		Backend:     "ebpf",
		Hook:        "cgroup-connect",
		Protocol:    "tcp",
		DstIP:       "203.0.113.10",
		DstPort:     443,
		Decision:    "block",
		Reason:      "domain-blocklist",
		MatchedRule: "*.segment.io",
	}
	if err := w.Write(ev); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	w.Close()

	data, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("reading events: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 event line, got %d", len(lines))
	}

	var parsed Event
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("parsing event JSON: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("parsing raw event JSON: %v", err)
	}
	if raw["schemaVersion"] == nil || raw["eventType"] != auditlog.EventNetworkConnect {
		t.Fatalf("expected audit envelope fields, got %v", raw)
	}

	if parsed.RunID != "run-1" {
		t.Errorf("unexpected runId: %s", parsed.RunID)
	}
	if parsed.Decision != "block" {
		t.Errorf("unexpected decision: %s", parsed.Decision)
	}
	if parsed.MatchedRule != "*.segment.io" {
		t.Errorf("unexpected matchedRule: %s", parsed.MatchedRule)
	}
	if parsed.TS == "" {
		t.Error("expected timestamp to be auto-populated")
	}
	if _, err := time.Parse(time.RFC3339, parsed.TS); err != nil {
		t.Errorf("timestamp is not RFC3339: %v", err)
	}
}

func TestDaemonEventWriterNoSecrets(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewDaemonEventWriter(hostPath, "", false, testRotation())
	if err != nil {
		t.Fatalf("NewDaemonEventWriter failed: %v", err)
	}

	ev := Event{
		RunID:    "run-1",
		Project:  "proj",
		Backend:  "ebpf",
		Protocol: "tcp",
		DstIP:    "10.0.0.1",
		DstPort:  80,
		Decision: "allow",
		Reason:   "default-allow",
	}
	if err := w.Write(ev); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	w.Close()

	data, _ := os.ReadFile(hostPath)
	content := string(data)

	forbidden := []string{"http://", "https://", "?q=", "authorization", "token", "secret", "password"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(content), f) {
			t.Errorf("event log should not contain %q", f)
		}
	}
}
