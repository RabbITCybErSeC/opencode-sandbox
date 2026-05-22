package config

import "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"

const (
	// DefaultImageName is the published runtime image used by fresh installs.
	DefaultImageName = "ghcr.io/rabbitcybersec/opencode-sandbox:latest"

	// DefaultStrictInitImage is the published strict eBPF init image.
	DefaultStrictInitImage = "ghcr.io/rabbitcybersec/opencode-sandbox-init:latest"
)

// Defaults returns the built-in default configuration.
func Defaults() EffectiveConfig {
	return EffectiveConfig{
		Version: 1,
		Image: EffectiveImage{
			Name:            DefaultImageName,
			AutoBuild:       false,
			StrictInitImage: DefaultStrictInitImage,
			InstallTools:    true,
		},
		Project: EffectiveProject{
			Target:   "/workspace",
			ReadOnly: false,
		},
		OpenCode: EffectiveOpenCode{
			MountHostConfig: true,
			MountHostData:   true,
			GeneratedConfig: true,
			Autoupdate:      false,
		},
		Skills: EffectiveSkills{
			Include: []string{"*"},
		},
		Network: EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			ProxyPort:     18080,
			DNSPort:       15353,
			FailClosed:    true,
			LocalhostAccess: EffectiveLocalhostAccess{
				Enabled: false,
				IP:      "203.0.113.113",
				Domain:  "host.container.internal",
			},
			EBPF: EffectiveEBPF{
				InitImage: DefaultStrictInitImage,
			},
		},
		Audit: EffectiveAudit{
			Rotation: EffectiveAuditRotation{
				MaxBytes: audit.DefaultRotationMaxBytes,
				MaxFiles: audit.DefaultRotationMaxFiles,
			},
			Commands: EffectiveCommandAudit{
				Enabled:             false,
				Backend:             "ebpf",
				FailClosed:          false,
				LogArgs:             "full",
				MaxArgs:             64,
				MaxArgBytes:         16384,
				IncludeExecutables:  []string{},
				ExcludeExecutables:  []string{},
				IncludeCwd:          []string{},
				ExcludeCwd:          []string{},
				MirrorProjectEvents: false,
			},
		},
		Resources: EffectiveResources{
			CPUs:   4,
			Memory: "4g",
		},
		Container: EffectiveContainer{
			NamePrefix: "opencode-sandbox",
			Remove:     true,
			Debug:      false,
		},
	}
}
