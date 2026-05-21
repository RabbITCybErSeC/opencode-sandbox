package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and strictly parses a single config file.
func Load(path string) (File, error) {
	f, err := os.Open(path)
	if err != nil {
		return File{}, fmt.Errorf("opening config: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var cfg File
	if err := dec.Decode(&cfg); err != nil {
		return File{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

// FindProjectConfig searches upward from startDir for .opencode-sandbox.yaml.
func FindProjectConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, ".opencode-sandbox.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "opencode-sandbox", "config.yaml"), nil
}

// MergeEffective merges defaults with any number of File overlays.
// Later files override earlier ones according to merge semantics.
func MergeEffective(base EffectiveConfig, overlays ...File) EffectiveConfig {
	for _, overlay := range overlays {
		base = applyFile(base, overlay)
	}
	return base
}

func applyFile(base EffectiveConfig, file File) EffectiveConfig {
	if file.Version != nil {
		base.Version = *file.Version
	}
	if file.Image != nil {
		base.Image = applyImage(base.Image, *file.Image)
	}
	if file.Project != nil {
		base.Project = applyProject(base.Project, *file.Project)
	}
	if file.OpenCode != nil {
		base.OpenCode = applyOpenCode(base.OpenCode, *file.OpenCode)
	}
	if file.Skills != nil {
		base.Skills = applySkills(base.Skills, *file.Skills)
	}
	if file.Network != nil {
		base.Network = applyNetwork(base.Network, *file.Network)
	}
	if file.Audit != nil {
		base.Audit = applyAudit(base.Audit, *file.Audit)
	}
	if file.Resources != nil {
		base.Resources = applyResources(base.Resources, *file.Resources)
	}
	if file.Container != nil {
		base.Container = applyContainer(base.Container, *file.Container)
	}
	return base
}

func applyImage(base EffectiveImage, img Image) EffectiveImage {
	if img.Name != nil {
		base.Name = *img.Name
	}
	if img.AutoBuild != nil {
		base.AutoBuild = *img.AutoBuild
	}
	if img.StrictInitImage != nil {
		base.StrictInitImage = *img.StrictInitImage
	}
	if img.Base != nil {
		base.Base = *img.Base
	}
	if img.InstallTools != nil {
		base.InstallTools = *img.InstallTools
	}
	return base
}

func applyProject(base EffectiveProject, proj Project) EffectiveProject {
	if proj.Target != nil {
		base.Target = *proj.Target
	}
	if proj.ReadOnly != nil {
		base.ReadOnly = *proj.ReadOnly
	}
	if proj.ExtraMounts != nil {
		base.ExtraMounts = *proj.ExtraMounts
	}
	return base
}

func applyOpenCode(base EffectiveOpenCode, oc OpenCode) EffectiveOpenCode {
	if oc.MountHostConfig != nil {
		base.MountHostConfig = *oc.MountHostConfig
	}
	if oc.MountHostData != nil {
		base.MountHostData = *oc.MountHostData
	}
	if oc.GeneratedConfig != nil {
		base.GeneratedConfig = *oc.GeneratedConfig
	}
	if oc.Autoupdate != nil {
		base.Autoupdate = *oc.Autoupdate
	}
	return base
}

func applySkills(base EffectiveSkills, sk Skills) EffectiveSkills {
	if sk.ImportedDir != nil {
		base.ImportedDir = *sk.ImportedDir
	}
	if sk.Include != nil {
		base.Include = *sk.Include
	}
	if sk.Exclude != nil {
		base.Exclude = *sk.Exclude
	}
	return base
}

func applyNetwork(base EffectiveNetwork, net Network) EffectiveNetwork {
	if net.Mode != nil {
		base.Mode = *net.Mode
	}
	if net.Backend != nil {
		base.Backend = *net.Backend
	}
	if net.DefaultAction != nil {
		base.DefaultAction = *net.DefaultAction
	}
	if net.ProxyPort != nil {
		base.ProxyPort = *net.ProxyPort
	}
	if net.DNSPort != nil {
		base.DNSPort = *net.DNSPort
	}
	if net.FailClosed != nil {
		base.FailClosed = *net.FailClosed
	}
	if net.LocalhostAccess != nil {
		base.LocalhostAccess = applyLocalhostAccess(base.LocalhostAccess, *net.LocalhostAccess)
	}

	if net.EBPF != nil {
		base.EBPF = applyEBPF(base.EBPF, *net.EBPF)
	}

	// inheritGlobal defaults to true when not explicitly set.
	inheritGlobal := true
	if net.InheritGlobal != nil {
		inheritGlobal = *net.InheritGlobal
	}

	if net.Blocklist != nil {
		if inheritGlobal {
			base.Blocklist = append(base.Blocklist, *net.Blocklist...)
		} else {
			base.Blocklist = *net.Blocklist
		}
	}
	if net.Allowlist != nil {
		if inheritGlobal {
			base.Allowlist = append(base.Allowlist, *net.Allowlist...)
		} else {
			base.Allowlist = *net.Allowlist
		}
	}

	return base
}

func applyLocalhostAccess(base EffectiveLocalhostAccess, la LocalhostAccess) EffectiveLocalhostAccess {
	if la.Enabled != nil {
		base.Enabled = *la.Enabled
	}
	if la.IP != nil {
		base.IP = *la.IP
	}
	if la.Domain != nil {
		base.Domain = *la.Domain
	}
	return base
}

func applyEBPF(base EffectiveEBPF, eb EBPF) EffectiveEBPF {
	if eb.InitImage != nil {
		base.InitImage = *eb.InitImage
	}
	if eb.NetworkName != nil {
		base.NetworkName = *eb.NetworkName
	}
	if eb.EventLog != nil {
		base.EventLog = *eb.EventLog
	}
	if eb.MirrorProjectEvents != nil {
		base.MirrorProjectEvents = *eb.MirrorProjectEvents
	}
	return base
}

func applyAudit(base EffectiveAudit, audit Audit) EffectiveAudit {
	if audit.Commands != nil {
		base.Commands = applyCommandAudit(base.Commands, *audit.Commands)
	}
	return base
}

func applyCommandAudit(base EffectiveCommandAudit, audit CommandAudit) EffectiveCommandAudit {
	if audit.Enabled != nil {
		base.Enabled = *audit.Enabled
	}
	if audit.Backend != nil {
		base.Backend = *audit.Backend
	}
	if audit.FailClosed != nil {
		base.FailClosed = *audit.FailClosed
	}
	if audit.LogArgs != nil {
		base.LogArgs = *audit.LogArgs
	}
	if audit.MaxArgs != nil {
		base.MaxArgs = *audit.MaxArgs
	}
	if audit.MaxArgBytes != nil {
		base.MaxArgBytes = *audit.MaxArgBytes
	}
	if audit.IncludeExecutables != nil {
		base.IncludeExecutables = *audit.IncludeExecutables
	}
	if audit.ExcludeExecutables != nil {
		base.ExcludeExecutables = *audit.ExcludeExecutables
	}
	if audit.IncludeCwd != nil {
		base.IncludeCwd = *audit.IncludeCwd
	}
	if audit.ExcludeCwd != nil {
		base.ExcludeCwd = *audit.ExcludeCwd
	}
	if audit.MirrorProjectEvents != nil {
		base.MirrorProjectEvents = *audit.MirrorProjectEvents
	}
	if audit.EventLog != nil {
		base.EventLog = *audit.EventLog
	}
	return base
}

func applyResources(base EffectiveResources, res Resources) EffectiveResources {
	if res.CPUs != nil {
		base.CPUs = *res.CPUs
	}
	if res.Memory != nil {
		base.Memory = *res.Memory
	}
	return base
}

func applyContainer(base EffectiveContainer, ctr Container) EffectiveContainer {
	if ctr.NamePrefix != nil {
		base.NamePrefix = *ctr.NamePrefix
	}
	if ctr.Remove != nil {
		base.Remove = *ctr.Remove
	}
	if ctr.Debug != nil {
		base.Debug = *ctr.Debug
	}
	return base
}

// ResolvePaths makes relative paths absolute against the config file directory.
func ResolvePaths(cfg *EffectiveConfig, configDir string) {
	// Skills importedDir may contain ~ expansion.
	cfg.Skills.ImportedDir = expandTilde(cfg.Skills.ImportedDir)
	for i, m := range cfg.Project.ExtraMounts {
		cfg.Project.ExtraMounts[i] = expandTilde(m)
	}
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

var memoryRe = regexp.MustCompile(`(?i)^\d+[kmgt]?[b]?$`)

// ValidateMemory checks that a memory string is well-formed.
func ValidateMemory(s string) error {
	if s == "" {
		return nil
	}
	if !memoryRe.MatchString(s) {
		return fmt.Errorf("invalid memory value: %q", s)
	}
	return nil
}
