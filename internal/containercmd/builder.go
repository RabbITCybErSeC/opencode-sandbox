package containercmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

// Plan captures everything needed to build a container run command.
type Plan struct {
	ProjectPath       string
	StagingDir        string
	MergedSkillsDir   string
	EventLogDir       string
	OpenCodeConfigDir string
	OpenCodeDataDir   string
	OpenCodeStateDir  string
	Image             string
	OpenCodeArgs      []string
	Effective         config.EffectiveConfig
}

// BuildArgv generates the complete container run argv slice.
func BuildArgv(plan Plan) []string {
	cfg := plan.Effective
	argv := []string{"container", "run"}

	if cfg.Container.Remove {
		argv = append(argv, "--rm")
	}
	argv = append(argv, "--interactive", "--tty")
	argv = append(argv, "--read-only")
	argv = append(argv, "--workdir", cfg.Project.Target)

	if cfg.Resources.CPUs > 0 {
		argv = append(argv, "--cpus", fmt.Sprintf("%d", cfg.Resources.CPUs))
	}
	if cfg.Resources.Memory != "" {
		argv = append(argv, "--memory", cfg.Resources.Memory)
	}

	// Project mount
	argv = append(argv, mountArg(plan.ProjectPath, cfg.Project.Target, cfg.Project.ReadOnly)...)

	for _, mount := range cfg.Project.ExtraMounts {
		parts := strings.Split(mount, ":")
		if len(parts) < 2 {
			continue
		}
		readonly := len(parts) > 2 && parts[2] == "ro"
		argv = append(argv, mountArg(parts[0], parts[1], readonly)...)
	}

	// Staging mount
	argv = append(argv, mountArg(plan.StagingDir, "/sandbox", false)...)

	if plan.OpenCodeConfigDir != "" {
		argv = append(argv, mountArg(plan.OpenCodeConfigDir, "/sandbox/home/.config/opencode", false)...)
	}
	if cfg.OpenCode.MountHostData && plan.OpenCodeDataDir != "" {
		argv = append(argv, mountArg(plan.OpenCodeDataDir, "/sandbox/home/.local/share/opencode", false)...)
	}
	if cfg.OpenCode.MountHostData && plan.OpenCodeStateDir != "" {
		argv = append(argv, mountArg(plan.OpenCodeStateDir, "/sandbox/home/.local/state/opencode", false)...)
	}

	// Merged imported skills mount (read-only)
	if plan.MergedSkillsDir != "" {
		argv = append(argv, mountArg(plan.MergedSkillsDir, "/sandbox/home/.config/opencode/skills", true)...)
	}

	// Event log mount (durable host state, overrides staging logs subdir)
	if plan.EventLogDir != "" {
		argv = append(argv, mountArg(plan.EventLogDir, "/sandbox/logs", false)...)
	}

	// tmpfs mounts
	argv = append(argv, "--tmpfs", "/tmp")
	argv = append(argv, "--tmpfs", "/run")

	// Environment variables
	argv = append(argv, "--env", "HOME=/sandbox/home")
	argv = append(argv, "--env", "XDG_CONFIG_HOME=/sandbox/home/.config")
	argv = append(argv, "--env", "XDG_DATA_HOME=/sandbox/home/.local/share")
	argv = append(argv, "--env", fmt.Sprintf("OPENCODE_SANDBOX_NETWORK_MODE=%s", cfg.Network.Mode))
	argv = append(argv, "--env", fmt.Sprintf("OPENCODE_SANDBOX_NETWORK_BACKEND=%s", cfg.Network.Backend))
	argv = append(argv, "--env", "OPENCODE_SANDBOX_POLICY_FILE=/sandbox/policy.json")
	if cfg.Network.FailClosed {
		argv = append(argv, "--env", "OPENCODE_SANDBOX_FAIL_CLOSED=true")
	}

	// Proxy env vars for proxy backend
	if cfg.Network.Backend == "proxy" {
		proxy := fmt.Sprintf("http://127.0.0.1:%d", cfg.Network.ProxyPort)
		argv = append(argv, "--env", fmt.Sprintf("HTTP_PROXY=%s", proxy))
		argv = append(argv, "--env", fmt.Sprintf("HTTPS_PROXY=%s", proxy))
		argv = append(argv, "--env", fmt.Sprintf("ALL_PROXY=%s", proxy))
		argv = append(argv, "--env", "NO_PROXY=localhost,127.0.0.1,::1")
	}

	if cfg.Network.Mode == "off" && cfg.Network.Backend != "proxy" {
		argv = append(argv, "--network", "none")
	}

	if cfg.Network.LocalhostAccess.Enabled {
		argv = append(argv, "--localhost", cfg.Network.LocalhostAccess.IP)
		argv = append(argv, "--env", fmt.Sprintf("OPENCODE_SANDBOX_HOST_DOMAIN=%s", cfg.Network.LocalhostAccess.Domain))
		argv = append(argv, "--env", fmt.Sprintf("OPENCODE_SANDBOX_HOST_IP=%s", cfg.Network.LocalhostAccess.IP))
	}

	// The init image hosts both network eBPF enforcement and command audit eBPF tracing.
	if initImage := RequiredInitImage(cfg); initImage != "" {
		argv = append(argv, "--init-image", initImage)
	}
	if cfg.Network.Backend == "ebpf" {
		if cfg.Network.Mode != "off" && cfg.Network.EBPF.NetworkName != "" {
			argv = append(argv, "--network", cfg.Network.EBPF.NetworkName)
		}
	}

	// Container name
	name := fmt.Sprintf("%s-%s", cfg.Container.NamePrefix, randomSuffix())
	argv = append(argv, "--name", name)

	// Image
	argv = append(argv, cfg.Image.Name)

	// The image entrypoint starts OpenCode; argv after the image is forwarded
	// directly to that entrypoint.
	argv = append(argv, plan.OpenCodeArgs...)

	return argv
}

// RequiredInitImage returns the init image needed by the selected runtime
// features. Command audit uses the same init image even in practical proxy mode.
func RequiredInitImage(cfg config.EffectiveConfig) string {
	if cfg.Network.Backend == "ebpf" || cfg.Audit.Commands.Enabled {
		return cfg.Network.EBPF.InitImage
	}
	return ""
}

func mountArg(source, target string, readonly bool) []string {
	arg := fmt.Sprintf("type=bind,source=%s,target=%s", source, target)
	if readonly {
		arg += ",readonly"
	}
	return []string{"--mount", arg}
}

func randomSuffix() string {
	// Use process ID + a counter for deterministic but unique names in tests.
	return fmt.Sprintf("%d", os.Getpid())
}

// RedactedCommand returns a human-readable command string with secrets hidden.
func RedactedCommand(argv []string) string {
	out := make([]string, len(argv))
	for i := 0; i < len(argv); i++ {
		out[i] = argv[i]
		if argv[i] == "--env" && i+1 < len(argv) {
			out[i+1] = redactSecret(argv[i+1])
			i++
		}
	}
	return strings.Join(out, " ")
}

func redactSecret(arg string) string {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return arg
	}
	name := parts[0]
	upper := strings.ToUpper(name)
	for _, keyword := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "AUTH"} {
		if strings.Contains(upper, keyword) {
			return name + "=***REDACTED***"
		}
	}
	return arg
}
