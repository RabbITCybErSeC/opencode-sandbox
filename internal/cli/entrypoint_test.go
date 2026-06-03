package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntrypointStopsPolicyProxyAndPropagatesOpenCodeExit(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "events.log")
	policyPath := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"mode":"practical"}`), 0644); err != nil {
		t.Fatal(err)
	}

	writeExecutable(t, filepath.Join(binDir, "policy-proxy"), `#!/bin/sh
echo "proxy-start:$1" >> "$TEST_ENTRYPOINT_LOG"
trap 'echo "proxy-term" >> "$TEST_ENTRYPOINT_LOG"; exit 0' TERM INT
while :; do sleep 1; done
`)
	writeExecutable(t, filepath.Join(binDir, "opencode"), `#!/bin/sh
echo "opencode:$*" >> "$TEST_ENTRYPOINT_LOG"
exit 7
`)

	cmd := exec.Command("bash", filepath.Join("..", "..", "entrypoint.sh"), "hello")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TEST_ENTRYPOINT_LOG="+logPath,
		"OPENCODE_SANDBOX_HOME="+filepath.Join(dir, "home"),
		"OPENCODE_SANDBOX_POLICY_FILE="+policyPath,
		"OPENCODE_SANDBOX_NETWORK_MODE=practical",
		"OPENCODE_SANDBOX_NETWORK_BACKEND=proxy",
	)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected opencode exit status to propagate")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("expected exit code 7, got err=%v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, want := range []string{"proxy-start:" + policyPath, "opencode:hello", "proxy-term"} {
		if !strings.Contains(log, want) {
			t.Fatalf("expected log to contain %q, got:\n%s", want, log)
		}
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
