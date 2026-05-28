package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

// PolicyFile describes the runtime network policy JSON (legacy, for proxy compat).
type PolicyFile struct {
	Mode          string   `json:"mode"`
	Backend       string   `json:"backend"`
	DefaultAction string   `json:"defaultAction"`
	Blocklist     []string `json:"blocklist"`
	Allowlist     []string `json:"allowlist"`
}

// PolicyBundle is the richer runtime policy bundle for eBPF and proxy modes.
type PolicyBundle struct {
	Version  int            `json:"version"`
	RunID    string         `json:"runId"`
	Project  ProjectInfo    `json:"project"`
	Network  NetworkConfig  `json:"network"`
	Audit    AuditConfig    `json:"audit"`
	Rules    Rules          `json:"rules"`
	Resolver ResolverConfig `json:"resolver"`
	Events   EventsConfig   `json:"events"`

	// Backward-compatibility fields so the existing proxy can still read
	// blocklist/allowlist and mode from the top level.
	Mode          string   `json:"mode"`
	DefaultAction string   `json:"defaultAction"`
	Blocklist     []string `json:"blocklist"`
	Allowlist     []string `json:"allowlist"`
}

// ProjectInfo holds project metadata in the bundle.
type ProjectInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// NetworkConfig holds network settings in the bundle.
type NetworkConfig struct {
	Mode          string `json:"mode"`
	Backend       string `json:"backend"`
	DefaultAction string `json:"defaultAction"`
	FailClosed    bool   `json:"failClosed"`
}

// AuditConfig holds audit settings in the bundle.
type AuditConfig struct {
	Events   AuditEventsConfig  `json:"events"`
	Commands CommandAuditConfig `json:"commands"`
}

// AuditEventsConfig holds unified audit event log settings.
type AuditEventsConfig struct {
	HostJsonl           string               `json:"hostJsonl"`
	ProjectMirrorJsonl  string               `json:"projectMirrorJsonl"`
	MirrorProjectEvents bool                 `json:"mirrorProjectEvents"`
	Rotation            audit.RotationConfig `json:"rotation"`
}

// CommandAuditConfig holds command execution audit settings.
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

// Rules holds domain and CIDR rules.
type Rules struct {
	BlockDomains []string `json:"blockDomains"`
	AllowDomains []string `json:"allowDomains"`
	BlockCIDRs   []string `json:"blockCIDRs"`
	AllowCIDRs   []string `json:"allowCIDRs"`
}

// ResolverConfig holds resolver tuning.
type ResolverConfig struct {
	TTLMinSeconds int `json:"ttlMinSeconds"`
	TTLMaxSeconds int `json:"ttlMaxSeconds"`
}

// EventsConfig holds event logging destinations.
type EventsConfig struct {
	HostJsonl           string `json:"hostJsonl"`
	ProjectMirrorJsonl  string `json:"projectMirrorJsonl"`
	MirrorProjectEvents bool   `json:"mirrorProjectEvents"`
}

// GeneratePolicyFile writes the legacy network policy JSON into the staging dir.
// Deprecated: use GeneratePolicyBundle for new code.
func GeneratePolicyFile(stagingDir string, mode, backend, defaultAction string, blocklist, allowlist []string) error {
	policy := PolicyFile{
		Mode:          mode,
		Backend:       backend,
		DefaultAction: defaultAction,
		Blocklist:     blocklist,
		Allowlist:     allowlist,
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling policy: %w", err)
	}
	path := filepath.Join(stagingDir, "policy.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing policy file: %w", err)
	}
	return nil
}

// GeneratePolicyBundle writes the rich policy bundle to /sandbox/policy.json.
func GeneratePolicyBundle(stagingDir, runID, projectPath, projectName string, cfg config.EffectiveConfig) error {
	bundle := PolicyBundle{
		Version: 1,
		RunID:   runID,
		Project: ProjectInfo{
			Path: projectPath,
			Name: projectName,
		},
		Network: NetworkConfig{
			Mode:          cfg.Network.Mode,
			Backend:       cfg.Network.Backend,
			DefaultAction: cfg.Network.DefaultAction,
			FailClosed:    cfg.Network.FailClosed,
		},
		Audit: AuditConfig{
			Events: AuditEventsConfig{
				HostJsonl:           "/sandbox/logs/" + audit.DefaultFileName,
				ProjectMirrorJsonl:  "/workspace/.opencode-sandbox/" + audit.DefaultFileName,
				MirrorProjectEvents: cfg.Network.EBPF.MirrorProjectEvents || cfg.Audit.Commands.MirrorProjectEvents,
				Rotation: audit.RotationConfig{
					MaxBytes: cfg.Audit.Rotation.MaxBytes,
					MaxFiles: cfg.Audit.Rotation.MaxFiles,
				},
			},
			Commands: CommandAuditConfig{
				Enabled:             cfg.Audit.Commands.Enabled,
				Backend:             cfg.Audit.Commands.Backend,
				FailClosed:          cfg.Audit.Commands.FailClosed,
				LogArgs:             cfg.Audit.Commands.LogArgs,
				MaxArgs:             cfg.Audit.Commands.MaxArgs,
				MaxArgBytes:         cfg.Audit.Commands.MaxArgBytes,
				IncludeExecutables:  cfg.Audit.Commands.IncludeExecutables,
				ExcludeExecutables:  cfg.Audit.Commands.ExcludeExecutables,
				IncludeCwd:          cfg.Audit.Commands.IncludeCwd,
				ExcludeCwd:          cfg.Audit.Commands.ExcludeCwd,
				MirrorProjectEvents: cfg.Audit.Commands.MirrorProjectEvents,
				HostJsonl:           "/sandbox/logs/" + audit.DefaultFileName,
				ProjectMirrorJsonl:  "/workspace/.opencode-sandbox/" + audit.DefaultFileName,
			},
		},
		Rules: Rules{
			BlockDomains: cfg.Network.Blocklist,
			AllowDomains: cfg.Network.Allowlist,
			BlockCIDRs:   []string{},
			AllowCIDRs:   []string{},
		},
		Resolver: ResolverConfig{
			TTLMinSeconds: 30,
			TTLMaxSeconds: 300,
		},
		Events: EventsConfig{
			HostJsonl:           "/sandbox/logs/" + audit.DefaultFileName,
			ProjectMirrorJsonl:  "/workspace/.opencode-sandbox/" + audit.DefaultFileName,
			MirrorProjectEvents: cfg.Network.EBPF.MirrorProjectEvents,
		},
		// Backward compatibility for the proxy.
		Mode:          cfg.Network.Mode,
		DefaultAction: cfg.Network.DefaultAction,
		Blocklist:     cfg.Network.Blocklist,
		Allowlist:     cfg.Network.Allowlist,
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling policy bundle: %w", err)
	}
	path := filepath.Join(stagingDir, "policy.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing policy bundle: %w", err)
	}
	return nil
}
