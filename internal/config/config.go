package config

// File is the pointer-backed config struct for YAML parsing.
// A nil field means "not set in this file".
type File struct {
	Version   *int       `yaml:"version,omitempty"`
	Image     *Image     `yaml:"image,omitempty"`
	Project   *Project   `yaml:"project,omitempty"`
	OpenCode  *OpenCode  `yaml:"opencode,omitempty"`
	Skills    *Skills    `yaml:"skills,omitempty"`
	Network   *Network   `yaml:"network,omitempty"`
	Audit     *Audit     `yaml:"audit,omitempty"`
	Resources *Resources `yaml:"resources,omitempty"`
	Container *Container `yaml:"container,omitempty"`
}

// Image holds container image settings.
type Image struct {
	Name            *string `yaml:"name,omitempty"`
	AutoBuild       *bool   `yaml:"autoBuild,omitempty"`
	StrictInitImage *string `yaml:"strictInitImage,omitempty"`
	Base            *string `yaml:"base,omitempty"`
	InstallTools    *bool   `yaml:"installTools,omitempty"`
}

// Project holds project mount settings.
type Project struct {
	Target      *string   `yaml:"target,omitempty"`
	ReadOnly    *bool     `yaml:"readonly,omitempty"`
	ExtraMounts *[]string `yaml:"extraMounts,omitempty"`
}

// OpenCode holds OpenCode-related settings.
type OpenCode struct {
	MountHostConfig *bool `yaml:"mountHostConfig,omitempty"`
	MountHostData   *bool `yaml:"mountHostData,omitempty"`
	GeneratedConfig *bool `yaml:"generatedConfig,omitempty"`
	Autoupdate      *bool `yaml:"autoupdate,omitempty"`
}

// Skills holds skill import settings.
type Skills struct {
	ImportedDir *string   `yaml:"importedDir,omitempty"`
	Include     *[]string `yaml:"include,omitempty"`
	Exclude     *[]string `yaml:"exclude,omitempty"`
}

// LocalhostAccess configures container-to-host networking.
type LocalhostAccess struct {
	Enabled *bool   `yaml:"enabled,omitempty"`
	IP      *string `yaml:"ip,omitempty"`
	Domain  *string `yaml:"domain,omitempty"`
}

// Network holds network policy settings.
type Network struct {
	InheritGlobal   *bool            `yaml:"inheritGlobal,omitempty"`
	Mode            *string          `yaml:"mode,omitempty"`
	Backend         *string          `yaml:"backend,omitempty"`
	DefaultAction   *string          `yaml:"defaultAction,omitempty"`
	Blocklist       *[]string        `yaml:"blocklist,omitempty"`
	Allowlist       *[]string        `yaml:"allowlist,omitempty"`
	ProxyPort       *int             `yaml:"proxyPort,omitempty"`
	DNSPort         *int             `yaml:"dnsPort,omitempty"`
	FailClosed      *bool            `yaml:"failClosed,omitempty"`
	LocalhostAccess *LocalhostAccess `yaml:"localhostAccess,omitempty"`
	EBPF            *EBPF            `yaml:"ebpf,omitempty"`
}

// EBPF holds eBPF backend-specific settings.
type EBPF struct {
	InitImage           *string `yaml:"initImage,omitempty"`
	NetworkName         *string `yaml:"networkName,omitempty"`
	EventLog            *string `yaml:"eventLog,omitempty"`
	MirrorProjectEvents *bool   `yaml:"mirrorProjectEvents,omitempty"`
}

// Audit holds observability and audit settings.
type Audit struct {
	EventLog *string        `yaml:"eventLog,omitempty"`
	Rotation *AuditRotation `yaml:"rotation,omitempty"`
	Commands *CommandAudit  `yaml:"commands,omitempty"`
}

// AuditRotation holds audit log rotation settings.
type AuditRotation struct {
	MaxBytes *int64 `yaml:"maxBytes,omitempty"`
	MaxFiles *int   `yaml:"maxFiles,omitempty"`
}

// CommandAudit holds command execution audit settings.
type CommandAudit struct {
	Enabled             *bool     `yaml:"enabled,omitempty"`
	Backend             *string   `yaml:"backend,omitempty"`
	FailClosed          *bool     `yaml:"failClosed,omitempty"`
	LogArgs             *string   `yaml:"logArgs,omitempty"`
	MaxArgs             *int      `yaml:"maxArgs,omitempty"`
	MaxArgBytes         *int      `yaml:"maxArgBytes,omitempty"`
	IncludeExecutables  *[]string `yaml:"includeExecutables,omitempty"`
	ExcludeExecutables  *[]string `yaml:"excludeExecutables,omitempty"`
	IncludeCwd          *[]string `yaml:"includeCwd,omitempty"`
	ExcludeCwd          *[]string `yaml:"excludeCwd,omitempty"`
	MirrorProjectEvents *bool     `yaml:"mirrorProjectEvents,omitempty"`
	EventLog            *string   `yaml:"eventLog,omitempty"`
}

// Resources holds container resource limits.
type Resources struct {
	CPUs   *int    `yaml:"cpus,omitempty"`
	Memory *string `yaml:"memory,omitempty"`
}

// Container holds container runtime settings.
type Container struct {
	NamePrefix *string `yaml:"namePrefix,omitempty"`
	Remove     *bool   `yaml:"remove,omitempty"`
	Debug      *bool   `yaml:"debug,omitempty"`
}

// EffectiveConfig is the fully-resolved, immutable runtime configuration.
type EffectiveConfig struct {
	Version   int
	Image     EffectiveImage
	Project   EffectiveProject
	OpenCode  EffectiveOpenCode
	Skills    EffectiveSkills
	Network   EffectiveNetwork
	Audit     EffectiveAudit
	Resources EffectiveResources
	Container EffectiveContainer
}

// EffectiveImage is the resolved image configuration.
type EffectiveImage struct {
	Name            string
	AutoBuild       bool
	StrictInitImage string
	Base            string
	InstallTools    bool
}

// EffectiveProject is the resolved project configuration.
type EffectiveProject struct {
	Target      string
	ReadOnly    bool
	ExtraMounts []string
}

// EffectiveOpenCode is the resolved OpenCode configuration.
type EffectiveOpenCode struct {
	MountHostConfig bool
	MountHostData   bool
	GeneratedConfig bool
	Autoupdate      bool
}

// EffectiveSkills is the resolved skills configuration.
type EffectiveSkills struct {
	ImportedDir string
	Include     []string
	Exclude     []string
}

// EffectiveLocalhostAccess is the resolved localhost access configuration.
type EffectiveLocalhostAccess struct {
	Enabled bool
	IP      string
	Domain  string
}

// EffectiveNetwork is the resolved network configuration.
type EffectiveNetwork struct {
	Mode            string
	Backend         string
	DefaultAction   string
	Blocklist       []string
	Allowlist       []string
	ProxyPort       int
	DNSPort         int
	FailClosed      bool
	LocalhostAccess EffectiveLocalhostAccess
	EBPF            EffectiveEBPF
}

// EffectiveEBPF is the resolved eBPF configuration.
type EffectiveEBPF struct {
	InitImage           string
	NetworkName         string
	EventLog            string
	MirrorProjectEvents bool
}

// EffectiveAudit is the resolved audit configuration.
type EffectiveAudit struct {
	EventLog string
	Rotation EffectiveAuditRotation
	Commands EffectiveCommandAudit
}

// EffectiveAuditRotation is the resolved audit log rotation configuration.
type EffectiveAuditRotation struct {
	MaxBytes int64
	MaxFiles int
}

// EffectiveCommandAudit is the resolved command execution audit configuration.
type EffectiveCommandAudit struct {
	Enabled             bool
	Backend             string
	FailClosed          bool
	LogArgs             string
	MaxArgs             int
	MaxArgBytes         int
	IncludeExecutables  []string
	ExcludeExecutables  []string
	IncludeCwd          []string
	ExcludeCwd          []string
	MirrorProjectEvents bool
	EventLog            string
}

// EffectiveResources is the resolved resource configuration.
type EffectiveResources struct {
	CPUs   int
	Memory string
}

// EffectiveContainer is the resolved container configuration.
type EffectiveContainer struct {
	NamePrefix string
	Remove     bool
	Debug      bool
}
