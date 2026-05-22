package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEventWriterCreatesHostFile(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewEventWriter(hostPath, "")
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(hostPath); err != nil {
		t.Errorf("expected host file to exist: %v", err)
	}
}

func TestEventWriterCreatesMirrorFile(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "host.jsonl")
	mirrorPath := filepath.Join(dir, "mirror.jsonl")

	w, err := NewEventWriter(hostPath, mirrorPath)
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(mirrorPath); err != nil {
		t.Errorf("expected mirror file to exist: %v", err)
	}
}

func TestEventWriterEmitsValidJSONL(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewEventWriter(hostPath, "")
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}

	ev := NetworkEvent{
		RunID:    "run-1",
		Project:  "myproj",
		Backend:  "ebpf",
		Hook:     "cgroup-connect",
		Protocol: "tcp",
		DstIP:    "203.0.113.10",
		DstPort:  443,
		Decision: "allow",
		Reason:   "default-allow",
	}
	if err := w.WriteEvent(ev); err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
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

	var parsed NetworkEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("parsing event JSON: %v", err)
	}

	if parsed.RunID != "run-1" {
		t.Errorf("unexpected runId: %s", parsed.RunID)
	}
	if parsed.DstIP != "203.0.113.10" {
		t.Errorf("unexpected dstIp: %s", parsed.DstIP)
	}
	if parsed.Decision != "allow" {
		t.Errorf("unexpected decision: %s", parsed.Decision)
	}
	if parsed.TS == "" {
		t.Error("expected timestamp to be auto-populated")
	}
	if _, err := time.Parse(time.RFC3339, parsed.TS); err != nil {
		t.Errorf("timestamp is not RFC3339: %v", err)
	}
}

func TestEventWriterDoesNotLogSensitiveData(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "network-events.jsonl")

	w, err := NewEventWriter(hostPath, "")
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}

	// Write an event that might accidentally contain a URL in a field.
	ev := NetworkEvent{
		RunID:       "run-1",
		Project:     "proj",
		Backend:     "ebpf",
		Hook:        "cgroup-connect",
		Protocol:    "tcp",
		DstIP:       "203.0.113.10",
		DstPort:     443,
		Decision:    "block",
		Reason:      "domain-blocklist",
		MatchedRule: "*.segment.io",
	}
	if err := w.WriteEvent(ev); err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}
	w.Close()

	data, _ := os.ReadFile(hostPath)
	content := string(data)

	forbidden := []string{"http://", "https://", "?q=", "authorization", "token", "secret"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(content), f) {
			t.Errorf("event log should not contain %q", f)
		}
	}
}

func TestEventLogDir(t *testing.T) {
	dir, err := EventLogDir("run-abc123")
	if err != nil {
		t.Fatalf("EventLogDir failed: %v", err)
	}
	if !strings.Contains(dir, "opencode-sandbox/runs/run-abc123") {
		t.Errorf("unexpected event log dir: %s", dir)
	}
}

func TestEventLogDirForBaseUsesConfiguredBase(t *testing.T) {
	dir, err := EventLogDirForBase("run-abc123", "/tmp/custom-events")
	if err != nil {
		t.Fatalf("EventLogDirForBase failed: %v", err)
	}
	if dir != "/tmp/custom-events/run-abc123" {
		t.Fatalf("unexpected event log dir: %s", dir)
	}
}

func TestEventLogDirForBaseExpandsHome(t *testing.T) {
	dir, err := EventLogDirForBase("run-abc123", "~/.local/state/custom")
	if err != nil {
		t.Fatalf("EventLogDirForBase failed: %v", err)
	}
	if strings.Contains(dir, "~") {
		t.Fatalf("expected home expansion, got %s", dir)
	}
	if !strings.HasSuffix(dir, ".local/state/custom/run-abc123") {
		t.Fatalf("unexpected event log dir: %s", dir)
	}
}

func TestEventLogBaseDirDefault(t *testing.T) {
	dir, err := EventLogBaseDir("")
	if err != nil {
		t.Fatalf("EventLogBaseDir failed: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".local", "state", "opencode-sandbox", "runs")) {
		t.Fatalf("unexpected base dir: %s", dir)
	}
}

func TestEventWriterMirror(t *testing.T) {
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "host.jsonl")
	mirrorPath := filepath.Join(dir, "mirror.jsonl")

	w, err := NewEventWriter(hostPath, mirrorPath)
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}

	ev := NetworkEvent{
		RunID:    "run-1",
		Project:  "proj",
		Backend:  "ebpf",
		Protocol: "tcp",
		DstIP:    "10.0.0.1",
		DstPort:  80,
		Decision: "allow",
		Reason:   "default-allow",
	}
	if err := w.WriteEvent(ev); err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}
	w.Close()

	hostData, _ := os.ReadFile(hostPath)
	mirrorData, _ := os.ReadFile(mirrorPath)
	if string(hostData) != string(mirrorData) {
		t.Error("expected host and mirror to have identical content")
	}
}
