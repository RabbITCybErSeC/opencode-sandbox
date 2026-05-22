package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

const (
	commandHookExecve    = "execve"
	commandHookExecveat  = "execveat"
	commandHookSchedExec = "sched_process_exec"

	commandSyscallExecve    = uint32(1)
	commandSyscallExecveat  = uint32(2)
	commandSyscallSchedExec = uint32(3)
)

type bpfCommandEvent struct {
	PID     uint32
	UID     uint32
	Syscall uint32
	_       uint32
}

// CommandMonitor owns eBPF exec tracepoints and the userspace event loop.
type CommandMonitor struct {
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	rd        *perf.Reader
	links     []link.Link
	progs     []*ebpf.Program
	events    *ebpf.Map
	writer    *DaemonEventWriter
	ownWriter bool
}

// StartCommandMonitor attaches execve/execveat tracepoints and starts logging.
func StartCommandMonitor(bundle *PolicyBundle, sharedWriter ...*DaemonEventWriter) (*CommandMonitor, error) {
	cfg := bundle.Audit.Commands
	if cfg.HostJsonl == "" {
		return nil, fmt.Errorf("command audit hostJsonl is empty")
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("removing memlock limit: %w", err)
	}

	var writer *DaemonEventWriter
	ownWriter := false
	if len(sharedWriter) > 0 && sharedWriter[0] != nil {
		writer = sharedWriter[0]
	} else {
		var err error
		writer, err = NewDaemonEventWriter(cfg.HostJsonl, cfg.ProjectMirrorJsonl, cfg.MirrorProjectEvents, bundle.Audit.Events.Rotation)
		if err != nil {
			return nil, fmt.Errorf("creating command event writer: %w", err)
		}
		ownWriter = true
	}

	events, err := ebpf.NewMap(&ebpf.MapSpec{
		Type: ebpf.PerfEventArray,
		Name: "command_events",
	})
	if err != nil {
		if ownWriter {
			writer.Close()
		}
		return nil, fmt.Errorf("creating command event map: %w", err)
	}

	rd, err := perf.NewReader(events, os.Getpagesize())
	if err != nil {
		events.Close()
		if ownWriter {
			writer.Close()
		}
		return nil, fmt.Errorf("creating command event reader: %w", err)
	}

	monitor := &CommandMonitor{
		rd:        rd,
		events:    events,
		writer:    writer,
		ownWriter: ownWriter,
	}
	if err := monitor.attachTracepoint("sched", commandHookSchedExec, commandHookSchedExec, commandSyscallSchedExec); err != nil {
		monitor.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	monitor.cancel = cancel
	monitor.wg.Add(1)
	go monitor.run(ctx, bundle)

	return monitor, nil
}

func (m *CommandMonitor) attachTracepoint(category, event, hook string, syscall uint32) error {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name:         "command_audit_" + hook,
		Type:         ebpf.TracePoint,
		License:      "GPL",
		Instructions: commandAuditInstructions(m.events.FD(), syscall),
	})
	if err != nil {
		return fmt.Errorf("loading command audit %s program: %w", hook, err)
	}
	m.progs = append(m.progs, prog)

	tp, err := link.Tracepoint(category, event, prog, nil)
	if err != nil {
		return fmt.Errorf("attaching command audit %s tracepoint: %w", hook, err)
	}
	m.links = append(m.links, tp)
	return nil
}

func commandAuditInstructions(eventsFD int, syscall uint32) asm.Instructions {
	return asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),

		asm.FnGetCurrentPidTgid.Call(),
		asm.RSh.Imm(asm.R0, 32),
		asm.StoreMem(asm.RFP, -16, asm.R0, asm.Word),

		asm.FnGetCurrentUidGid.Call(),
		asm.StoreMem(asm.RFP, -12, asm.R0, asm.Word),

		asm.Mov.Imm(asm.R0, int32(syscall)),
		asm.StoreMem(asm.RFP, -8, asm.R0, asm.Word),
		asm.Mov.Imm(asm.R0, 0),
		asm.StoreMem(asm.RFP, -4, asm.R0, asm.Word),

		asm.Mov.Reg(asm.R1, asm.R6),
		asm.LoadMapPtr(asm.R2, eventsFD),
		asm.LoadImm(asm.R3, 0xffffffff, asm.DWord),
		asm.Mov.Reg(asm.R4, asm.RFP),
		asm.Add.Imm(asm.R4, -16),
		asm.Mov.Imm(asm.R5, 16),
		asm.FnPerfEventOutput.Call(),

		asm.Mov.Imm(asm.R0, 0),
		asm.Return(),
	}
}

func (m *CommandMonitor) run(ctx context.Context, bundle *PolicyBundle) {
	defer m.wg.Done()
	for {
		record, err := m.rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Fprintf(os.Stderr, "policy-ebpfd: reading command audit event: %v\n", err)
				continue
			}
		}
		if record.LostSamples > 0 {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: lost %d command audit samples\n", record.LostSamples)
			_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "command-audit", Reason: "lost-samples", LostSamples: record.LostSamples})
		}
		ev, err := commandEventFromSample(record.RawSample, bundle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: parsing command audit event: %v\n", err)
			_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "command-audit", Reason: "parse-error", Error: err.Error()})
			continue
		}
		if ev == nil {
			continue
		}
		if err := m.writer.WriteCommand(*ev); err != nil {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: writing command audit event: %v\n", err)
			_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "command-audit", Reason: "write-error", Error: err.Error()})
		}
	}
}

func commandEventFromSample(raw []byte, bundle *PolicyBundle) (*CommandEvent, error) {
	if len(raw) < 16 {
		return nil, fmt.Errorf("short command event sample: %d bytes", len(raw))
	}
	ev := bpfCommandEvent{
		PID:     binary.LittleEndian.Uint32(raw[0:4]),
		UID:     binary.LittleEndian.Uint32(raw[4:8]),
		Syscall: binary.LittleEndian.Uint32(raw[8:12]),
	}
	hook := commandHookExecve
	switch ev.Syscall {
	case commandSyscallExecveat:
		hook = commandHookExecveat
	case commandSyscallSchedExec:
		hook = commandHookSchedExec
	}
	enriched := enrichCommandEvent(int(ev.PID), int(ev.UID), hook, bundle)
	if !commandEventAllowed(enriched, bundle.Audit.Commands) {
		return nil, nil
	}
	return enriched, nil
}

func enrichCommandEvent(pid, uid int, hook string, bundle *PolicyBundle) *CommandEvent {
	cfg := bundle.Audit.Commands
	info := readProcInfo(pid, cfg)
	if info.UID == 0 {
		info.UID = uid
	}
	return &CommandEvent{
		RunID:     bundle.RunID,
		Project:   bundle.Project.Name,
		Backend:   "ebpf",
		Hook:      hook,
		PID:       pid,
		PPID:      info.PPID,
		UID:       info.UID,
		GID:       info.GID,
		CWD:       info.CWD,
		Exe:       info.Exe,
		Argv:      info.Argv,
		Argc:      info.Argc,
		Truncated: info.Truncated,
		Decision:  "allow",
		Reason:    "command-audit",
	}
}

type procCommandInfo struct {
	PPID      int
	UID       int
	GID       int
	CWD       string
	Exe       string
	Argv      []string
	Argc      int
	Truncated bool
}

func readProcInfo(pid int, cfg CommandAuditConfig) procCommandInfo {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	info := procCommandInfo{}
	info.Exe, _ = os.Readlink(filepath.Join(base, "exe"))
	info.CWD, _ = os.Readlink(filepath.Join(base, "cwd"))
	info.PPID, info.UID, info.GID = readProcStatus(filepath.Join(base, "status"))
	if cfg.LogArgs != "none" {
		info.Argv, info.Argc, info.Truncated = readProcCmdline(filepath.Join(base, "cmdline"), cfg.MaxArgs, cfg.MaxArgBytes)
	}
	return info
}

func readProcStatus(path string) (ppid, uid, gid int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "PPid:"):
			ppid = parseFirstInt(line)
		case strings.HasPrefix(line, "Uid:"):
			uid = parseFirstInt(line)
		case strings.HasPrefix(line, "Gid:"):
			gid = parseFirstInt(line)
		}
	}
	return ppid, uid, gid
}

func parseFirstInt(line string) int {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(fields[1])
	return value
}

func readProcCmdline(path string, maxArgs, maxBytes int) ([]string, int, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, 0, false
	}
	parts := bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0})
	argc := len(parts)
	argv := make([]string, 0, argc)
	truncated := false
	usedBytes := 0
	for _, part := range parts {
		if maxArgs > 0 && len(argv) >= maxArgs {
			truncated = true
			break
		}
		if maxBytes > 0 && usedBytes+len(part) > maxBytes {
			truncated = true
			break
		}
		argv = append(argv, string(part))
		usedBytes += len(part)
	}
	return argv, argc, truncated
}

func commandEventAllowed(ev *CommandEvent, cfg CommandAuditConfig) bool {
	exe := ev.Exe
	if exe == "" && len(ev.Argv) > 0 {
		exe = ev.Argv[0]
	}
	if len(cfg.IncludeExecutables) > 0 && !matchesAnyExecutable(exe, cfg.IncludeExecutables) {
		return false
	}
	if matchesAnyExecutable(exe, cfg.ExcludeExecutables) {
		return false
	}
	if len(cfg.IncludeCwd) > 0 && !matchesAnyPath(ev.CWD, cfg.IncludeCwd) {
		return false
	}
	if matchesAnyPath(ev.CWD, cfg.ExcludeCwd) {
		return false
	}
	return true
}

func matchesAnyExecutable(exe string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesExecutable(exe, pattern) {
			return true
		}
	}
	return false
}

func matchesExecutable(exe, pattern string) bool {
	if exe == pattern || filepath.Base(exe) == pattern {
		return true
	}
	if ok, _ := filepath.Match(pattern, exe); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, filepath.Base(exe)); ok {
		return true
	}
	return false
}

func matchesAnyPath(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if path == pattern || strings.HasPrefix(path, strings.TrimRight(pattern, "/")+"/") {
			return true
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
	}
	return false
}

// Close detaches tracepoints and closes all event resources.
func (m *CommandMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.rd != nil {
		_ = m.rd.Close()
	}
	m.wg.Wait()

	var firstErr error
	for _, l := range m.links {
		if err := l.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, prog := range m.progs {
		if err := prog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if m.events != nil {
		if err := m.events.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if m.writer != nil && m.ownWriter {
		if err := m.writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
