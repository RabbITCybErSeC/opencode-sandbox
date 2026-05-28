// policy-ebpfd is the strict-init policy daemon.
// It runs inside the Apple container VM before the main OCI process starts.
// It parses the policy bundle, probes eBPF capabilities, attempts to attach
// an egress hook, and enforces fail-closed behavior when required.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
)

// PolicyBundle mirrors the runtime bundle shape.
type PolicyBundle struct {
	Version int    `json:"version"`
	RunID   string `json:"runId"`
	Project struct {
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"project"`
	Network struct {
		Mode          string `json:"mode"`
		Backend       string `json:"backend"`
		DefaultAction string `json:"defaultAction"`
		FailClosed    bool   `json:"failClosed"`
	} `json:"network"`
	Audit struct {
		Events   AuditEventsConfig  `json:"events"`
		Commands CommandAuditConfig `json:"commands"`
	} `json:"audit"`
	Rules struct {
		BlockDomains []string `json:"blockDomains"`
		AllowDomains []string `json:"allowDomains"`
		BlockCIDRs   []string `json:"blockCIDRs"`
		AllowCIDRs   []string `json:"allowCIDRs"`
	} `json:"rules"`
	Resolver struct {
		TTLMinSeconds int `json:"ttlMinSeconds"`
		TTLMaxSeconds int `json:"ttlMaxSeconds"`
	} `json:"resolver"`
	Events struct {
		HostJsonl           string `json:"hostJsonl"`
		ProjectMirrorJsonl  string `json:"projectMirrorJsonl"`
		MirrorProjectEvents bool   `json:"mirrorProjectEvents"`
	} `json:"events"`

	// Backward compatibility fields.
	Mode      string   `json:"mode"`
	Blocklist []string `json:"blocklist"`
	Allowlist []string `json:"allowlist"`
}

// AuditEventsConfig mirrors the unified audit event bundle shape.
type AuditEventsConfig struct {
	HostJsonl           string                  `json:"hostJsonl"`
	ProjectMirrorJsonl  string                  `json:"projectMirrorJsonl"`
	MirrorProjectEvents bool                    `json:"mirrorProjectEvents"`
	Rotation            auditlog.RotationConfig `json:"rotation"`
}

// CommandAuditConfig mirrors the command audit bundle shape.
type CommandAuditConfig struct {
	Enabled             bool     `json:"enabled"`
	Backend             string   `json:"backend"`
	FailClosed          bool     `json:"failClosed"`
	LogArgs             string   `json:"logArgs"`
	MaxArgs             int      `json:"maxArgs"`
	MaxArgBytes         int      `json:"maxArgBytes"`
	IncludeExecutables  []string `json:"includeExecutables"`
	ExcludeExecutables  []string `json:"excludeExecutables"`
	IncludeCwd          []string `json:"includeCwd"`
	ExcludeCwd          []string `json:"excludeCwd"`
	MirrorProjectEvents bool     `json:"mirrorProjectEvents"`
	HostJsonl           string   `json:"hostJsonl"`
	ProjectMirrorJsonl  string   `json:"projectMirrorJsonl"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "policy-ebpfd: %v\n", err)
		os.Exit(1)
	}

	// Block forever so the init daemon stays alive.
	// In a full implementation this would wait on a signal or context.
	select {}
}

func run() error {
	policyPath := os.Getenv("OPENCODE_SANDBOX_POLICY_FILE")
	if policyPath == "" {
		policyPath = "/sandbox/policy.json"
	}

	bundle, err := loadPolicyBundle(policyPath)
	if err != nil {
		return fmt.Errorf("loading policy bundle: %w", err)
	}

	if err := validateBundle(bundle); err != nil {
		return fmt.Errorf("validating policy bundle: %w", err)
	}
	normalizeAuditEventConfig(bundle)

	fmt.Printf("policy-ebpfd: runId=%s project=%s mode=%s backend=%s defaultAction=%s\n",
		bundle.RunID, bundle.Project.Name, bundle.Network.Mode,
		bundle.Network.Backend, bundle.Network.DefaultAction)

	eventWriter, err := NewDaemonEventWriter(
		bundle.Audit.Events.HostJsonl,
		bundle.Audit.Events.ProjectMirrorJsonl,
		bundle.Audit.Events.MirrorProjectEvents,
		bundle.Audit.Events.Rotation,
	)
	if err != nil {
		return fmt.Errorf("creating event writer: %w", err)
	}
	defer eventWriter.Close()
	_ = eventWriter.WriteAudit(healthEvent(bundle, "daemon", true, "daemon-start"))

	// Probe available eBPF capabilities.
	caps := probeCapabilities()
	fmt.Printf("policy-ebpfd: cgroup2 available=%v\n", caps.Cgroup2Available)

	var enforcer *enforcementHandle
	if bundle.Network.Backend == "ebpf" {
		// Attempt to attach the preferred cgroup/connect hook.
		// If unavailable and failClosed is true, exit non-zero.
		var attachErr error
		enforcer, attachErr = tryAttachHooks(caps, bundle)
		if attachErr != nil {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: network hook attach failed: %v\n", attachErr)
			_ = eventWriter.WriteAudit(healthEvent(bundle, "network-hook", false, attachErr.Error()))
			if bundle.Network.FailClosed {
				return fmt.Errorf("fail-closed: cannot establish eBPF enforcement")
			}
			fmt.Println("policy-ebpfd: continuing without eBPF network enforcement (failClosed=false)")
		} else {
			defer enforcer.Close()
			fmt.Println("policy-ebpfd: eBPF network hook attached successfully")
		}
	}

	var commandMonitor *CommandMonitor
	if bundle.Audit.Commands.Enabled {
		monitor, err := StartCommandMonitor(bundle, eventWriter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: command audit attach failed: %v\n", err)
			_ = eventWriter.WriteAudit(healthEvent(bundle, "command-audit", false, err.Error()))
			if bundle.Audit.Commands.FailClosed {
				return fmt.Errorf("fail-closed: cannot establish command audit")
			}
			fmt.Println("policy-ebpfd: continuing without command audit (failClosed=false)")
		} else {
			commandMonitor = monitor
			defer commandMonitor.Close()
			_ = commandMonitor.writer.WriteAudit(healthEvent(bundle, "command-audit", true, "attached"))
			fmt.Println("policy-ebpfd: eBPF command audit attached successfully")
		}
	}

	if enforcer != nil {
		_ = eventWriter.WriteAudit(healthEvent(bundle, "network-hook", true, "attached"))
		networkMonitor, err := StartNetworkEventMonitor(bundle, enforcer, eventWriter)
		if err != nil {
			_ = eventWriter.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "network-connect", Reason: "attach-failed", Error: err.Error()})
		} else {
			defer networkMonitor.Close()
			_ = eventWriter.WriteAudit(healthEvent(bundle, "network-connect", true, "reader-started"))
		}
	}

	if bundle.Network.Backend == "ebpf" {
		// Start resolver/control loop.
		var mgr MapManager
		if enforcer != nil {
			mgr = enforcer
		} else {
			mgr = newStubMapManager()
		}
		resolver := NewResolver(
			bundle.Rules.BlockDomains,
			bundle.Rules.AllowDomains,
			bundle.Resolver.TTLMinSeconds,
			bundle.Resolver.TTLMaxSeconds,
			mgr,
		)
		go func() {
			ctx := context.Background()
			if err := resolver.Run(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "resolver: %v\n", err)
				_ = eventWriter.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "resolver", Error: err.Error()})
			}
		}()
		_ = eventWriter.WriteAudit(healthEvent(bundle, "resolver", true, "started"))
		fmt.Println("policy-ebpfd: resolver started")
	}

	_ = eventWriter.WriteAudit(healthEvent(bundle, "daemon", true, "daemon-ready"))
	fmt.Println("policy-ebpfd: daemon ready")
	return nil
}

func normalizeAuditEventConfig(bundle *PolicyBundle) {
	if bundle.Audit.Events.HostJsonl == "" {
		bundle.Audit.Events.HostJsonl = bundle.Events.HostJsonl
	}
	if bundle.Audit.Events.ProjectMirrorJsonl == "" {
		bundle.Audit.Events.ProjectMirrorJsonl = bundle.Events.ProjectMirrorJsonl
	}
	if !bundle.Audit.Events.MirrorProjectEvents {
		bundle.Audit.Events.MirrorProjectEvents = bundle.Events.MirrorProjectEvents || bundle.Audit.Commands.MirrorProjectEvents
	}
	if bundle.Audit.Events.Rotation.MaxBytes == 0 {
		bundle.Audit.Events.Rotation.MaxBytes = auditlog.DefaultRotationMaxBytes
	}
	if bundle.Audit.Events.Rotation.MaxFiles == 0 {
		bundle.Audit.Events.Rotation.MaxFiles = auditlog.DefaultRotationMaxFiles
	}
	if bundle.Audit.Commands.HostJsonl == "" {
		bundle.Audit.Commands.HostJsonl = bundle.Audit.Events.HostJsonl
	}
	if bundle.Audit.Commands.ProjectMirrorJsonl == "" {
		bundle.Audit.Commands.ProjectMirrorJsonl = bundle.Audit.Events.ProjectMirrorJsonl
	}
}

func healthEvent(bundle *PolicyBundle, component string, active bool, message string) auditlog.Event {
	return auditlog.Event{
		EventType: auditlog.EventDaemonHealth,
		RunID:     bundle.RunID,
		Project:   bundle.Project.Name,
		Backend:   "ebpf",
		Component: component,
		Status:    message,
		Active:    &active,
		Attached:  &active,
		Message:   message,
	}
}

func writeStartupHealth(bundle *PolicyBundle, component string, active bool, message string) error {
	writer, err := NewDaemonEventWriter(
		bundle.Audit.Events.HostJsonl,
		bundle.Audit.Events.ProjectMirrorJsonl,
		bundle.Audit.Events.MirrorProjectEvents,
		bundle.Audit.Events.Rotation,
	)
	if err != nil {
		return err
	}
	defer writer.Close()
	return writer.WriteAudit(healthEvent(bundle, component, active, message))
}

func loadPolicyBundle(path string) (*PolicyBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bundle PolicyBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func validateBundle(bundle *PolicyBundle) error {
	if bundle.Version != 1 {
		return fmt.Errorf("unsupported bundle version: %d", bundle.Version)
	}
	if bundle.Network.Backend != "ebpf" && !bundle.Audit.Commands.Enabled {
		return fmt.Errorf("backend is %q and command audit is disabled; init daemon has nothing to do", bundle.Network.Backend)
	}
	if bundle.Network.Backend == "ebpf" && bundle.Network.Mode != "strict" && bundle.Network.Mode != "off" {
		return fmt.Errorf("mode %q is not supported with ebpf backend", bundle.Network.Mode)
	}
	if bundle.Audit.Commands.Enabled && bundle.Audit.Commands.Backend != "ebpf" {
		return fmt.Errorf("audit.commands backend is %q, expected ebpf", bundle.Audit.Commands.Backend)
	}
	return nil
}

// capabilities holds probed eBPF capability state.
type capabilities struct {
	Cgroup2Available bool
	CgroupPath       string
}

func probeCapabilities() capabilities {
	caps := capabilities{}
	cgroupV2Path := "/sys/fs/cgroup"
	if info, err := os.Stat(cgroupV2Path); err == nil && info.IsDir() {
		// Check if cgroup2 is actually mounted by looking for cgroup.controllers
		controllersPath := filepath.Join(cgroupV2Path, "cgroup.controllers")
		if _, err := os.Stat(controllersPath); err == nil {
			caps.Cgroup2Available = true
			caps.CgroupPath = cgroupV2Path
		}
	}
	return caps
}

// tryAttachHooks attempts to attach the preferred cgroup/connect program.
// If cgroup attach is unavailable, it documents the tc egress fallback path
// but does not implement it in this POC.
func tryAttachHooks(caps capabilities, bundle *PolicyBundle) (*enforcementHandle, error) {
	if !caps.Cgroup2Available {
		return nil, fmt.Errorf("cgroup2 not available; tc egress fallback not yet implemented")
	}
	return attachCgroupConnect(caps.CgroupPath, bundle)
}
