package doctor

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/containercmd"
)

// Check describes a single doctor check result.
type Check struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
	StatusSkip = "skip"
)

var inspectDoctorImage = func(image string) ([]byte, error) {
	return exec.Command("container", imageInspectArgs(image)...).CombinedOutput()
}

// Run performs all environment checks with the provided configuration.
// If cfg is the zero value, it loads global config or falls back to defaults.
func Run(cfg config.EffectiveConfig) []Check {
	var checks []Check

	checkContainerBinary(&checks)
	checkContainerVersion(&checks)
	checkContainerNetwork(&checks)
	checkInitImage(&checks, cfg)
	checkNetworkName(&checks, cfg)
	checkEBPFSupport(&checks, cfg)
	checkHostDNS(&checks, cfg)

	return checks
}

func checkContainerBinary(checks *[]Check) {
	path, err := exec.LookPath("container")
	if err != nil {
		*checks = append(*checks, Check{
			ID:      "container.binary",
			Status:  StatusFail,
			Message: "Apple container binary not found in PATH. Install Apple's container tool.",
		})
		return
	}
	*checks = append(*checks, Check{
		ID:      "container.binary",
		Status:  StatusPass,
		Message: fmt.Sprintf("container found at %s", path),
	})
}

func checkContainerVersion(checks *[]Check) {
	if _, err := exec.LookPath("container"); err != nil {
		*checks = append(*checks, Check{
			ID:      "container.version",
			Status:  StatusSkip,
			Message: "Skipping because container binary is missing",
		})
		return
	}
	out, err := exec.Command("container", "system", "version").CombinedOutput()
	if err != nil {
		*checks = append(*checks, Check{
			ID:      "container.version",
			Status:  StatusWarn,
			Message: fmt.Sprintf("container system version failed: %v", err),
		})
		return
	}
	*checks = append(*checks, Check{
		ID:      "container.version",
		Status:  StatusPass,
		Message: strings.TrimSpace(string(out)),
	})
}

func checkContainerNetwork(checks *[]Check) {
	if runtime.GOOS != "darwin" {
		*checks = append(*checks, Check{
			ID:      "container.network",
			Status:  StatusWarn,
			Message: "Apple container is only supported on macOS",
		})
		return
	}
	// macOS 26+ is recommended for custom container networks.
	// We can't easily parse the exact macOS version here without cgo,
	// so we do a coarse check.
	*checks = append(*checks, Check{
		ID:      "container.network",
		Status:  StatusPass,
		Message: "macOS detected; custom networks require macOS 26+",
	})
}

func checkInitImage(checks *[]Check, cfg config.EffectiveConfig) {
	initImage := containercmd.RequiredInitImage(cfg)
	if initImage == "" && cfg.Network.Backend != "ebpf" && !cfg.Audit.Commands.Enabled {
		*checks = append(*checks, Check{
			ID:      "ebpf.init-image",
			Status:  StatusSkip,
			Message: "No init image required",
		})
		return
	}
	if initImage == "" {
		*checks = append(*checks, Check{
			ID:      "ebpf.init-image",
			Status:  StatusFail,
			Message: "Init image is required by eBPF networking or command audit, but no init image is specified",
		})
		return
	}
	// Check if the image exists locally via container image inspect.
	if _, err := inspectDoctorImage(initImage); err != nil {
		*checks = append(*checks, Check{
			ID:      "ebpf.init-image",
			Status:  StatusWarn,
			Message: fmt.Sprintf("init image %q not found locally: pull with `opencode-sandbox image pull --strict-init`, build with `opencode-sandbox image build --strict-init`, or disable command audit if you do not need it", initImage),
		})
		return
	}
	*checks = append(*checks, Check{
		ID:      "ebpf.init-image",
		Status:  StatusPass,
		Message: fmt.Sprintf("init image %q found", initImage),
	})
}

func imageInspectArgs(image string) []string {
	return []string{"image", "inspect", image}
}

func checkNetworkName(checks *[]Check, cfg config.EffectiveConfig) {
	if cfg.Network.Backend != "ebpf" {
		*checks = append(*checks, Check{
			ID:      "ebpf.network-name",
			Status:  StatusSkip,
			Message: "eBPF backend not configured",
		})
		return
	}
	if cfg.Network.EBPF.NetworkName == "" {
		*checks = append(*checks, Check{
			ID:      "ebpf.network-name",
			Status:  StatusWarn,
			Message: "No custom network name configured; default network will be used",
		})
		return
	}
	// Check if the network exists via container network inspect.
	if _, err := exec.Command("container", "network", "inspect", cfg.Network.EBPF.NetworkName).CombinedOutput(); err != nil {
		*checks = append(*checks, Check{
			ID:      "ebpf.network-name",
			Status:  StatusWarn,
			Message: fmt.Sprintf("network %q not found: create with `container network create %s`", cfg.Network.EBPF.NetworkName, cfg.Network.EBPF.NetworkName),
		})
		return
	}
	*checks = append(*checks, Check{
		ID:      "ebpf.network-name",
		Status:  StatusPass,
		Message: fmt.Sprintf("network %q found", cfg.Network.EBPF.NetworkName),
	})
}

func checkHostDNS(checks *[]Check, cfg config.EffectiveConfig) {
	if !cfg.Network.LocalhostAccess.Enabled {
		*checks = append(*checks, Check{
			ID:      "host.dns",
			Status:  StatusSkip,
			Message: "localhostAccess is not enabled",
		})
		return
	}

	if runtime.GOOS != "darwin" {
		*checks = append(*checks, Check{
			ID:      "host.dns",
			Status:  StatusWarn,
			Message: "host DNS check is only supported on macOS",
		})
		return
	}

	domain := cfg.Network.LocalhostAccess.Domain
	if domain == "" {
		domain = "host.container.internal"
	}

	out, err := exec.Command("dscacheutil", "-q", "host", "-a", "name", domain).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "name:") {
		ip := cfg.Network.LocalhostAccess.IP
		if ip == "" {
			ip = "203.0.113.113"
		}
		*checks = append(*checks, Check{
			ID:      "host.dns",
			Status:  StatusWarn,
			Message: fmt.Sprintf("DNS domain %q is not configured on the host. Run: sudo container system dns create %s --localhost %s", domain, domain, ip),
		})
		return
	}

	*checks = append(*checks, Check{
		ID:      "host.dns",
		Status:  StatusPass,
		Message: fmt.Sprintf("DNS domain %q resolves on host", domain),
	})
}

func checkEBPFSupport(checks *[]Check, cfg config.EffectiveConfig) {
	if cfg.Network.Backend != "ebpf" {
		*checks = append(*checks, Check{
			ID:      "ebpf.support",
			Status:  StatusSkip,
			Message: "eBPF backend not configured; practical proxy mode is available",
		})
		return
	}
	if runtime.GOOS != "darwin" {
		*checks = append(*checks, Check{
			ID:      "ebpf.support",
			Status:  StatusFail,
			Message: "eBPF strict mode is only supported on macOS with Apple container",
		})
		return
	}
	*checks = append(*checks, Check{
		ID:      "ebpf.support",
		Status:  StatusPass,
		Message: "strict init image configured; full eBPF support requires Apple container VM with cgroup2",
	})
}

// IsHealthy returns true when no check is in fail state.
func IsHealthy(checks []Check) bool {
	for _, c := range checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}
