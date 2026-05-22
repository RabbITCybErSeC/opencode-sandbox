package main

import (
	"fmt"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
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
	writer *auditlog.Writer
}

// NewDaemonEventWriter opens the event sinks configured in the bundle.
func NewDaemonEventWriter(hostPath, mirrorPath string, mirror bool, rotation auditlog.RotationConfig) (*DaemonEventWriter, error) {
	writer, err := auditlog.NewWriter(hostPath, mirrorPath, mirror, rotation)
	if err != nil {
		return nil, err
	}
	return &DaemonEventWriter{writer: writer}, nil
}

// Write emits a single event to all configured sinks.
func (w *DaemonEventWriter) Write(ev Event) error {
	return w.writer.Write(auditlog.Event{
		EventType:   auditlog.EventNetworkConnect,
		RunID:       ev.RunID,
		Project:     ev.Project,
		Backend:     ev.Backend,
		Hook:        ev.Hook,
		PID:         ev.PID,
		Process:     ev.Process,
		Protocol:    ev.Protocol,
		DstIP:       ev.DstIP,
		DstPort:     ev.DstPort,
		Decision:    ev.Decision,
		Reason:      ev.Reason,
		MatchedRule: ev.MatchedRule,
	})
}

// WriteCommand emits a single command audit event to all configured sinks.
func (w *DaemonEventWriter) WriteCommand(ev CommandEvent) error {
	return w.writer.Write(auditlog.Event{
		EventType: auditlog.EventCommandExec,
		RunID:     ev.RunID,
		Project:   ev.Project,
		Backend:   ev.Backend,
		Hook:      ev.Hook,
		PID:       ev.PID,
		PPID:      ev.PPID,
		UID:       ev.UID,
		GID:       ev.GID,
		CWD:       ev.CWD,
		Exe:       ev.Exe,
		Argv:      ev.Argv,
		Argc:      ev.Argc,
		Truncated: ev.Truncated,
		Decision:  ev.Decision,
		Reason:    ev.Reason,
	})
}

func (w *DaemonEventWriter) WriteAudit(ev auditlog.Event) error {
	if ev.EventType == "" {
		return fmt.Errorf("audit event type is empty")
	}
	return w.writer.Write(ev)
}

// Close flushes and closes all sinks.
func (w *DaemonEventWriter) Close() error {
	return w.writer.Close()
}
