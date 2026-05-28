package config

import (
	"fmt"
	"net"
	"strings"
)

// Validate checks that the effective configuration is well-formed.
func Validate(cfg EffectiveConfig) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	if cfg.Network.Mode != "" && cfg.Network.Mode != "practical" && cfg.Network.Mode != "strict" && cfg.Network.Mode != "off" {
		return fmt.Errorf("invalid network mode: %s", cfg.Network.Mode)
	}
	if cfg.Network.Backend != "" && cfg.Network.Backend != "proxy" && cfg.Network.Backend != "ebpf" {
		return fmt.Errorf("invalid network backend: %s", cfg.Network.Backend)
	}
	if cfg.Network.DefaultAction != "" && cfg.Network.DefaultAction != "allow" && cfg.Network.DefaultAction != "deny" {
		return fmt.Errorf("invalid network defaultAction: %s", cfg.Network.DefaultAction)
	}
	if cfg.Network.Mode == "off" && cfg.Network.DefaultAction != "deny" {
		return fmt.Errorf("network mode 'off' requires defaultAction 'deny'")
	}
	if cfg.Network.Mode == "off" && cfg.Network.Backend == "proxy" {
		return fmt.Errorf("network mode 'off' is not supported with proxy backend; use ebpf strict backend")
	}
	if cfg.Network.Backend == "ebpf" && cfg.Network.EBPF.InitImage == "" {
		return fmt.Errorf("network backend 'ebpf' requires a non-empty ebpf.initImage")
	}
	if cfg.Audit.Commands.Enabled {
		if cfg.Audit.Commands.Backend != "ebpf" {
			return fmt.Errorf("audit.commands backend %q is not supported; use ebpf", cfg.Audit.Commands.Backend)
		}
		if cfg.Network.EBPF.InitImage == "" {
			return fmt.Errorf("audit.commands enabled requires a non-empty network.ebpf.initImage")
		}
		if cfg.Audit.Commands.LogArgs != "full" && cfg.Audit.Commands.LogArgs != "none" {
			return fmt.Errorf("invalid audit.commands.logArgs: %s", cfg.Audit.Commands.LogArgs)
		}
		if cfg.Audit.Commands.MaxArgs < 0 {
			return fmt.Errorf("invalid audit.commands.maxArgs: %d", cfg.Audit.Commands.MaxArgs)
		}
		if cfg.Audit.Commands.MaxArgBytes < 0 {
			return fmt.Errorf("invalid audit.commands.maxArgBytes: %d", cfg.Audit.Commands.MaxArgBytes)
		}
		for _, entry := range cfg.Audit.Commands.IncludeExecutables {
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty audit.commands.includeExecutables entry")
			}
		}
		for _, entry := range cfg.Audit.Commands.ExcludeExecutables {
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty audit.commands.excludeExecutables entry")
			}
		}
		for _, entry := range cfg.Audit.Commands.IncludeCwd {
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty audit.commands.includeCwd entry")
			}
		}
		for _, entry := range cfg.Audit.Commands.ExcludeCwd {
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty audit.commands.excludeCwd entry")
			}
		}
	}
	if cfg.Audit.Rotation.MaxBytes < 0 {
		return fmt.Errorf("invalid audit.rotation.maxBytes: %d", cfg.Audit.Rotation.MaxBytes)
	}
	if cfg.Audit.Rotation.MaxFiles < 0 {
		return fmt.Errorf("invalid audit.rotation.maxFiles: %d", cfg.Audit.Rotation.MaxFiles)
	}
	if cfg.Network.LocalhostAccess.Enabled {
		if cfg.Network.LocalhostAccess.IP == "" {
			return fmt.Errorf("localhostAccess.enabled requires a non-empty ip")
		}
		if net.ParseIP(cfg.Network.LocalhostAccess.IP) == nil {
			return fmt.Errorf("invalid localhostAccess.ip: %s", cfg.Network.LocalhostAccess.IP)
		}
		if cfg.Network.LocalhostAccess.Domain == "" {
			return fmt.Errorf("localhostAccess.enabled requires a non-empty domain")
		}
	}
	if cfg.Resources.CPUs < 0 {
		return fmt.Errorf("invalid cpus: %d", cfg.Resources.CPUs)
	}
	if err := ValidateMemory(cfg.Resources.Memory); err != nil {
		return err
	}
	for _, d := range cfg.Network.Blocklist {
		if strings.TrimSpace(d) == "" {
			return fmt.Errorf("empty blocklist entry")
		}
	}
	for _, d := range cfg.Network.Allowlist {
		if strings.TrimSpace(d) == "" {
			return fmt.Errorf("empty allowlist entry")
		}
	}
	for _, mount := range cfg.Project.ExtraMounts {
		parts := strings.Split(mount, ":")
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return fmt.Errorf("invalid extra mount %q, expected host:container[:ro]", mount)
		}
		if len(parts) > 2 && parts[2] != "ro" && parts[2] != "rw" {
			return fmt.Errorf("invalid extra mount mode %q in %q", parts[2], mount)
		}
	}
	return nil
}
