package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepairOpenCodeStateRepairsMalformedAuth(t *testing.T) {
	paths := testStatePaths(t)
	authPath := filepath.Join(paths.DataDir, "auth.json")
	if err := os.WriteFile(authPath, []byte("   "), 0600); err != nil {
		t.Fatal(err)
	}
	withFixedOpenCodeStateTime(t)

	report, err := RepairOpenCodeState(paths)
	if err != nil {
		t.Fatalf("RepairOpenCodeState failed: %v", err)
	}
	if report.Auth.Status != OpenCodeStateStatusRepaired {
		t.Fatalf("expected auth repaired, got %+v", report.Auth)
	}
	if report.Auth.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "{}" {
		t.Fatalf("expected auth reset to {}, got %q", data)
	}
	if _, err := os.Stat(report.Auth.BackupPath); err != nil {
		t.Fatalf("expected auth backup to exist: %v", err)
	}
}

func TestRepairOpenCodeStateQuarantinesCorruptDBWithWalAndShm(t *testing.T) {
	paths := testStatePaths(t)
	dbPath := filepath.Join(paths.DataDir, "opencode.db")
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.WriteFile(path, []byte("broken"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	withFixedOpenCodeStateTime(t)
	withSQLitePragma(t, func(path, pragma string) (string, error) {
		return "database disk image is malformed", errors.New("sqlite corrupt")
	})

	report, err := RepairOpenCodeState(paths)
	if err != nil {
		t.Fatalf("RepairOpenCodeState failed: %v", err)
	}
	if report.Database.Status != OpenCodeStateStatusRepaired {
		t.Fatalf("expected database repaired, got %+v", report.Database)
	}
	for _, name := range []string{"opencode.db", "opencode.db-wal", "opencode.db-shm"} {
		if _, err := os.Stat(filepath.Join(report.Database.BackupPath, name)); err != nil {
			t.Fatalf("expected %s in backup: %v", name, err)
		}
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected original db removed, got err=%v", err)
	}
}

func TestCheckOpenCodeStateMissingFilesPasses(t *testing.T) {
	paths := testStatePaths(t)
	report := CheckOpenCodeState(paths)
	if report.Auth.Status != OpenCodeStateStatusPass {
		t.Fatalf("expected missing auth to pass, got %+v", report.Auth)
	}
	if report.Database.Status != OpenCodeStateStatusPass {
		t.Fatalf("expected missing db to pass, got %+v", report.Database)
	}
}

func TestMaintainOpenCodeStateCheckpointsHealthyDB(t *testing.T) {
	paths := testStatePaths(t)
	dbPath := filepath.Join(paths.DataDir, "opencode.db")
	if err := os.WriteFile(dbPath, []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}
	var pragmas []string
	withSQLitePragma(t, func(path, pragma string) (string, error) {
		pragmas = append(pragmas, pragma)
		return "ok", nil
	})

	if err := MaintainOpenCodeState(paths); err != nil {
		t.Fatalf("MaintainOpenCodeState failed: %v", err)
	}
	if len(pragmas) != 2 || pragmas[0] != "PRAGMA quick_check;" || pragmas[1] != "PRAGMA wal_checkpoint(TRUNCATE);" {
		t.Fatalf("unexpected pragmas: %v", pragmas)
	}
}

func TestDiagnoseOpenCodeStartupLogsSummarizesMalformedDB(t *testing.T) {
	paths := testStatePaths(t)
	logDir := filepath.Join(paths.DataDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "2026-06-03T060144.log")
	log := "ERROR service=server error=database disk image is malformed\nAffected startup requests: config.providers, provider.list\n"
	if err := os.WriteFile(logPath, []byte(log), 0644); err != nil {
		t.Fatal(err)
	}

	msg, ok := DiagnoseOpenCodeStartupLogs(paths)
	if !ok {
		t.Fatal("expected diagnosis")
	}
	if !strings.Contains(msg, "SQLite database") || !strings.Contains(msg, "config.providers") {
		t.Fatalf("unexpected diagnosis: %s", msg)
	}
}

func testStatePaths(t *testing.T) OpenCodeStatePaths {
	t.Helper()
	base := t.TempDir()
	paths := OpenCodeStatePaths{
		ConfigDir: filepath.Join(base, "config"),
		DataDir:   filepath.Join(base, "data"),
		StateDir:  filepath.Join(base, "state"),
	}
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.StateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func withFixedOpenCodeStateTime(t *testing.T) {
	t.Helper()
	old := openCodeStateNow
	openCodeStateNow = func() time.Time { return time.Date(2026, 6, 3, 6, 1, 44, 0, time.UTC) }
	t.Cleanup(func() { openCodeStateNow = old })
}

func withSQLitePragma(t *testing.T, fn func(path, pragma string) (string, error)) {
	t.Helper()
	old := runSQLitePragma
	runSQLitePragma = fn
	t.Cleanup(func() { runSQLitePragma = old })
}
