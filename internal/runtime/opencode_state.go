package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

const (
	OpenCodeStateStatusPass     = "pass"
	OpenCodeStateStatusWarn     = "warn"
	OpenCodeStateStatusRepaired = "repaired"
)

// OpenCodeStateItem describes one managed OpenCode state artifact.
type OpenCodeStateItem struct {
	Status     string
	Path       string
	Message    string
	BackupPath string
}

// OpenCodeStateReport summarizes managed OpenCode state health.
type OpenCodeStateReport struct {
	Auth     OpenCodeStateItem
	Database OpenCodeStateItem
}

var (
	openCodeStateNow = time.Now
	runSQLitePragma  = defaultRunSQLitePragma
)

// UsesDurableOpenCodeData reports whether a run mounts persistent OpenCode data.
func UsesDurableOpenCodeData(cfg config.EffectiveConfig) bool {
	return cfg.OpenCode.MountHostData
}

// CheckOpenCodeState validates wrapper-managed OpenCode state without mutating it.
func CheckOpenCodeState(paths OpenCodeStatePaths) OpenCodeStateReport {
	return OpenCodeStateReport{
		Auth:     checkOpenCodeAuth(paths.DataDir),
		Database: checkOpenCodeDB(paths.DataDir),
	}
}

// RepairOpenCodeState backs up and repairs known-corrupt managed OpenCode state.
func RepairOpenCodeState(paths OpenCodeStatePaths) (OpenCodeStateReport, error) {
	report := CheckOpenCodeState(paths)
	var errs []error

	if isRepairableAuth(report.Auth) {
		item, err := repairOpenCodeAuth(report.Auth.Path)
		if err != nil {
			errs = append(errs, err)
		} else {
			report.Auth = item
		}
	}

	if isRepairableDB(report.Database) {
		item, err := quarantineOpenCodeDB(paths.DataDir)
		if err != nil {
			errs = append(errs, err)
		} else {
			report.Database = item
		}
	}

	return report, errors.Join(errs...)
}

// MaintainOpenCodeState performs best-effort post-run SQLite maintenance.
func MaintainOpenCodeState(paths OpenCodeStatePaths) error {
	dbPath := openCodeDBPath(paths.DataDir)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	item := checkOpenCodeDB(paths.DataDir)
	if item.Status != OpenCodeStateStatusPass {
		return nil
	}
	if _, err := runSQLitePragma(dbPath, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("checkpointing OpenCode database: %w", err)
	}
	return nil
}

// RepairMessages returns user-facing repair summaries.
func (r OpenCodeStateReport) RepairMessages() []string {
	var messages []string
	for _, item := range []OpenCodeStateItem{r.Auth, r.Database} {
		if item.Status == OpenCodeStateStatusRepaired {
			messages = append(messages, item.Message)
		}
	}
	return messages
}

// DiagnoseOpenCodeStartupLogs summarizes known opaque OpenCode startup failures.
// Only logs written at or after since are considered relevant to the current run.
func DiagnoseOpenCodeStartupLogs(paths OpenCodeStatePaths, since time.Time) (string, bool) {
	logPath, ok := latestOpenCodeLog(filepath.Join(paths.DataDir, "log"), since)
	if !ok {
		return "", false
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", false
	}
	text := string(data)
	lower := strings.ToLower(text)

	var cause string
	switch {
	case strings.Contains(lower, "database disk image is malformed"):
		cause = "OpenCode's managed SQLite database appears to be corrupt"
	case strings.Contains(text, "Auth.all") && strings.Contains(lower, "json parse"):
		cause = "OpenCode's auth.json appears to be malformed"
	case strings.Contains(text, "ConfigInvalidError"):
		cause = "OpenCode's config appears to be invalid"
	default:
		return "", false
	}

	affected := firstLineContaining(text, "Affected startup requests:")
	if affected != "" {
		return fmt.Sprintf("%s. Latest log: %s. %s", cause, logPath, strings.TrimSpace(affected)), true
	}
	return fmt.Sprintf("%s. Latest log: %s.", cause, logPath), true
}

func checkOpenCodeAuth(dataDir string) OpenCodeStateItem {
	path := filepath.Join(dataDir, "auth.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return OpenCodeStateItem{Status: OpenCodeStateStatusPass, Path: path, Message: "auth.json is not present yet"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: fmt.Sprintf("cannot read auth.json: %v", err)}
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: "auth.json is empty or whitespace-only"}
	}
	var auth map[string]json.RawMessage
	if err := json.Unmarshal(data, &auth); err != nil {
		return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: fmt.Sprintf("auth.json is malformed: %v", err)}
	}
	return OpenCodeStateItem{Status: OpenCodeStateStatusPass, Path: path, Message: "auth.json is valid JSON"}
}

func checkOpenCodeDB(dataDir string) OpenCodeStateItem {
	path := openCodeDBPath(dataDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return OpenCodeStateItem{Status: OpenCodeStateStatusPass, Path: path, Message: "opencode.db is not present yet"}
	}
	out, err := runSQLitePragma(path, "PRAGMA quick_check;")
	if err != nil {
		if isSQLiteCorrupt(out, err) {
			return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: "opencode.db is malformed"}
		}
		return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: fmt.Sprintf("cannot verify opencode.db: %v", err)}
	}
	if strings.TrimSpace(out) != "ok" {
		return OpenCodeStateItem{Status: OpenCodeStateStatusWarn, Path: path, Message: fmt.Sprintf("opencode.db quick_check reported: %s", strings.TrimSpace(out))}
	}
	return OpenCodeStateItem{Status: OpenCodeStateStatusPass, Path: path, Message: "opencode.db passed quick_check"}
}

func repairOpenCodeAuth(path string) (OpenCodeStateItem, error) {
	backup := path + ".corrupt-" + timestamp()
	if err := os.Rename(path, backup); err != nil {
		return OpenCodeStateItem{}, fmt.Errorf("backing up malformed auth.json: %w", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		return OpenCodeStateItem{}, fmt.Errorf("resetting auth.json: %w", err)
	}
	return OpenCodeStateItem{
		Status:     OpenCodeStateStatusRepaired,
		Path:       path,
		BackupPath: backup,
		Message:    fmt.Sprintf("Repaired malformed OpenCode auth.json; backup saved at %s. Run `opencode auth login` inside the sandbox if credentials need refreshing.", backup),
	}, nil
}

func quarantineOpenCodeDB(dataDir string) (OpenCodeStateItem, error) {
	dbPath := openCodeDBPath(dataDir)
	backupDir := filepath.Join(dataDir, "corrupt-db-backups", timestamp())
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return OpenCodeStateItem{}, fmt.Errorf("creating corrupt DB backup dir: %w", err)
	}
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		target := filepath.Join(backupDir, filepath.Base(path))
		if err := os.Rename(path, target); err != nil {
			return OpenCodeStateItem{}, fmt.Errorf("quarantining %s: %w", path, err)
		}
	}
	return OpenCodeStateItem{
		Status:     OpenCodeStateStatusRepaired,
		Path:       dbPath,
		BackupPath: backupDir,
		Message:    fmt.Sprintf("Quarantined malformed OpenCode database; backup saved at %s. OpenCode will create a fresh database on next start.", backupDir),
	}, nil
}

func defaultRunSQLitePragma(dbPath, pragma string) (string, error) {
	out, err := exec.Command("sqlite3", dbPath, pragma).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func isRepairableAuth(item OpenCodeStateItem) bool {
	return item.Status == OpenCodeStateStatusWarn && (strings.Contains(item.Message, "malformed") || strings.Contains(item.Message, "whitespace-only"))
}

func isRepairableDB(item OpenCodeStateItem) bool {
	return item.Status == OpenCodeStateStatusWarn && strings.Contains(item.Message, "malformed")
}

func isSQLiteCorrupt(output string, err error) bool {
	text := strings.ToLower(output + " " + err.Error())
	return strings.Contains(text, "database disk image is malformed") || strings.Contains(text, "sqlite_corrupt")
}

func openCodeDBPath(dataDir string) string {
	return filepath.Join(dataDir, "opencode.db")
}

func timestamp() string {
	return openCodeStateNow().UTC().Format("20060102-150405")
}

func latestOpenCodeLog(logDir string, since time.Time) (string, bool) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", false
	}
	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		path := filepath.Join(logDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !since.IsZero() && info.ModTime().Before(since) {
			continue
		}
		candidates = append(candidates, candidate{path: path, modTime: info.ModTime().UnixNano()})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})
	if len(candidates) == 0 {
		return "", false
	}
	return candidates[0].path, true
}

func firstLineContaining(text, needle string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
