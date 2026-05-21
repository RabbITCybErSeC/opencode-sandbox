// vminit-wrapper preserves Apple container's VM init contract while starting
// the opencode-sandbox policy daemon when its runtime bundle is available.
package main

import (
	"fmt"
	"os"
	"syscall"
)

const (
	defaultPolicyPath = "/sandbox/policy.json"
	policyPathEnv     = "OPENCODE_SANDBOX_POLICY_FILE"
	policyDaemonPath  = "/usr/local/bin/policy-ebpfd"
	realVminitdPath   = "/sbin/vminitd.real"
)

func main() {
	policyPath := policyPathFromEnv(os.Getenv)
	if policyFileAvailable(os.Stat, policyPath) {
		if err := startPolicyDaemon(policyPath); err != nil {
			logf("failed to start policy daemon: %v", err)
		} else {
			logf("started policy daemon with %s", policyPath)
		}
	} else {
		logf("policy bundle %s unavailable; continuing without policy daemon", policyPath)
	}

	if err := syscall.Exec(realVminitdPath, os.Args, os.Environ()); err != nil {
		logf("failed to exec %s: %v", realVminitdPath, err)
		os.Exit(1)
	}
}

func policyPathFromEnv(getenv func(string) string) string {
	if path := getenv(policyPathEnv); path != "" {
		return path
	}
	return defaultPolicyPath
}

func policyFileAvailable(stat func(string) (os.FileInfo, error), path string) bool {
	info, err := stat(path)
	return err == nil && !info.IsDir()
}

func startPolicyDaemon(policyPath string) error {
	env := os.Environ()
	if os.Getenv(policyPathEnv) == "" {
		env = append(env, policyPathEnv+"="+policyPath)
	}

	proc, err := os.StartProcess(policyDaemonPath, []string{policyDaemonPath}, &os.ProcAttr{
		Env: env,
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
	})
	if err != nil {
		return err
	}
	return proc.Release()
}

func logf(format string, args ...any) {
	message := fmt.Sprintf("opencode-sandbox-init: "+format+"\n", args...)
	if kmsg, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0); err == nil {
		_, _ = kmsg.WriteString("<6>" + message)
		_ = kmsg.Close()
	}
	_, _ = os.Stderr.WriteString(message)
}
