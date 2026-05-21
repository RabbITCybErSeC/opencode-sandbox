package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReadProcCmdlineHonorsLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(path, []byte("curl\x00https://example.com\x00-H\x00Authorization: Bearer token\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	argv, argc, truncated := readProcCmdline(path, 2, 0)
	if argc != 4 {
		t.Fatalf("expected argc 4, got %d", argc)
	}
	if !truncated {
		t.Fatal("expected truncated=true")
	}
	if strings.Join(argv, " ") != "curl https://example.com" {
		t.Fatalf("unexpected argv: %v", argv)
	}
}

func TestReadProcCmdlineHonorsByteLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(path, []byte("curl\x00https://example.com\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	argv, argc, truncated := readProcCmdline(path, 64, 5)
	if argc != 2 {
		t.Fatalf("expected argc 2, got %d", argc)
	}
	if !truncated {
		t.Fatal("expected truncated=true")
	}
	if len(argv) != 1 || argv[0] != "curl" {
		t.Fatalf("unexpected argv: %v", argv)
	}
}

func TestReadProcStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status")
	content := "Name:\ttest\nPPid:\t42\nUid:\t1000\t1000\t1000\t1000\nGid:\t1001\t1001\t1001\t1001\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ppid, uid, gid := readProcStatus(path)
	if ppid != 42 || uid != 1000 || gid != 1001 {
		t.Fatalf("unexpected status values: ppid=%d uid=%d gid=%d", ppid, uid, gid)
	}
}

func TestCommandEventAllowedFiltersExecutableAndCwd(t *testing.T) {
	cfg := CommandAuditConfig{
		IncludeExecutables: []string{"curl"},
		ExcludeCwd:         []string{"/workspace/private"},
	}

	allowed := &CommandEvent{Exe: "/usr/bin/curl", CWD: "/workspace"}
	if !commandEventAllowed(allowed, cfg) {
		t.Fatal("expected curl in /workspace to be allowed")
	}

	blockedExe := &CommandEvent{Exe: "/usr/bin/git", CWD: "/workspace"}
	if commandEventAllowed(blockedExe, cfg) {
		t.Fatal("expected git to be filtered by includeExecutables")
	}

	blockedCwd := &CommandEvent{Exe: "/usr/bin/curl", CWD: "/workspace/private/repo"}
	if commandEventAllowed(blockedCwd, cfg) {
		t.Fatal("expected private cwd to be excluded")
	}
}

func TestCommandEventFromSampleUsesExecveatHook(t *testing.T) {
	bundle := &PolicyBundle{
		RunID: "run-1",
	}
	bundle.Project.Name = "proj"
	bundle.Audit.Commands = CommandAuditConfig{
		Enabled:     true,
		Backend:     "ebpf",
		LogArgs:     "none",
		MaxArgs:     64,
		MaxArgBytes: 16384,
	}

	raw := make([]byte, 16)
	binary.LittleEndian.PutUint32(raw[0:4], 123)
	binary.LittleEndian.PutUint32(raw[4:8], 1000)
	binary.LittleEndian.PutUint32(raw[8:12], commandSyscallExecveat)

	ev, err := commandEventFromSample(raw, bundle)
	if err != nil {
		t.Fatalf("commandEventFromSample failed: %v", err)
	}
	if ev == nil {
		t.Fatal("expected event")
	}
	if ev.Hook != commandHookExecveat {
		t.Fatalf("expected execveat hook, got %s", ev.Hook)
	}
	if ev.PID != 123 || ev.UID != 1000 || ev.Argc != 0 {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestLiveCommandMonitorCapturesExec(t *testing.T) {
	if os.Getenv("OPENCODE_SANDBOX_LIVE") != "1" {
		t.Skip("Set OPENCODE_SANDBOX_LIVE=1 to run live eBPF command audit test")
	}
	if runtime.GOOS != "linux" {
		t.Skip("live command monitor test requires Linux")
	}

	dir := t.TempDir()
	hostEvents := filepath.Join(dir, "command-events.jsonl")
	bundle := &PolicyBundle{
		Version: 1,
		RunID:   "run-live",
	}
	bundle.Project.Name = "proj"
	bundle.Audit.Commands = CommandAuditConfig{
		Enabled:     true,
		Backend:     "ebpf",
		FailClosed:  true,
		LogArgs:     "full",
		MaxArgs:     64,
		MaxArgBytes: 16384,
		HostJsonl:   hostEvents,
	}

	monitor, err := StartCommandMonitor(bundle)
	if err != nil {
		t.Fatalf("StartCommandMonitor failed: %v", err)
	}
	defer monitor.Close()

	if err := exec.Command("sh", "-c", "echo command-audit-live-test >/dev/null").Run(); err != nil {
		t.Fatalf("running test command: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(hostEvents)
		if strings.Contains(string(data), "command-audit-live-test") || strings.Contains(string(data), "/bin/sh") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	data, _ := os.ReadFile(hostEvents)
	t.Fatalf("expected command event in %s, got:\n%s", hostEvents, string(data))
}
