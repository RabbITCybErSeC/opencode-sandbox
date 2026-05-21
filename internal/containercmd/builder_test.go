package containercmd

import (
	"strings"
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func TestBuildArgvIncludesRequiredFlags(t *testing.T) {
	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        config.DefaultImageName,
		OpenCodeArgs: []string{"--help"},
		Effective:    config.Defaults(),
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	required := []string{
		"container run",
		"--rm",
		"--interactive",
		"--tty",
		"--read-only",
		"--workdir /workspace",
		"--cpus 4",
		"--memory 4g",
		"type=bind,source=/tmp/project,target=/workspace",
		"type=bind,source=/tmp/staging,target=/sandbox",
		"--tmpfs /tmp",
		"--tmpfs /run",
		"HOME=/sandbox/home",
		"XDG_CONFIG_HOME=/sandbox/home/.config",
		"OPENCODE_SANDBOX_NETWORK_MODE=practical",
		"OPENCODE_SANDBOX_NETWORK_BACKEND=proxy",
		config.DefaultImageName,
		"--help",
	}

	for _, r := range required {
		if !strings.Contains(joined, r) {
			t.Errorf("expected argv to contain %q, got:\n%s", r, joined)
		}
	}
}

func TestBuildArgvReliesOnImageEntrypoint(t *testing.T) {
	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        config.DefaultImageName,
		OpenCodeArgs: []string{"--help"},
		Effective:    config.Defaults(),
	}

	argv := BuildArgv(plan)
	if len(argv) < 2 {
		t.Fatalf("unexpected short argv: %v", argv)
	}
	if argv[len(argv)-2] != config.DefaultImageName {
		t.Fatalf("expected image before forwarded args, got tail: %v", argv[len(argv)-2:])
	}
	if argv[len(argv)-1] != "--help" {
		t.Fatalf("expected only forwarded OpenCode args after image, got tail: %v", argv[len(argv)-2:])
	}
}

func TestBuildArgvPracticalProxyEnv(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "practical"
	cfg.Network.Backend = "proxy"
	cfg.Network.ProxyPort = 18080

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "HTTP_PROXY=http://127.0.0.1:18080") {
		t.Error("expected HTTP_PROXY env var")
	}
	if !strings.Contains(joined, "HTTPS_PROXY=http://127.0.0.1:18080") {
		t.Error("expected HTTPS_PROXY env var")
	}
	if !strings.Contains(joined, "NO_PROXY=localhost,127.0.0.1,::1") {
		t.Error("expected NO_PROXY env var")
	}
}

func TestBuildArgvDefaultPracticalProxyOmitsInitImage(t *testing.T) {
	cfg := config.Defaults()
	plan := Plan{
		ProjectPath: "/tmp/project",
		StagingDir:  "/tmp/staging",
		Image:       "test:latest",
		Effective:   cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--init-image") {
		t.Fatalf("default practical proxy runs should not require an init image, got:\n%s", joined)
	}
}

func TestBuildArgvEbpfNoProxy(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "strict"
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "HTTP_PROXY") {
		t.Error("ebpf backend should not set proxy env vars")
	}
	if !strings.Contains(joined, "OPENCODE_SANDBOX_NETWORK_BACKEND=ebpf") {
		t.Error("expected OPENCODE_SANDBOX_NETWORK_BACKEND=ebpf env var")
	}
}

func TestBuildArgvEbpfInitImage(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "strict"
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--init-image opencode-sandbox-init:latest") {
		t.Error("expected --init-image flag")
	}
}

func TestBuildArgvCommandAuditInitImageWithProxyNetwork(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "practical"
	cfg.Network.Backend = "proxy"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	cfg.Audit.Commands.Enabled = true

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--init-image opencode-sandbox-init:latest") {
		t.Error("expected command audit to add --init-image even with proxy network backend")
	}
	if !strings.Contains(joined, "HTTP_PROXY=http://127.0.0.1:18080") {
		t.Error("expected proxy env vars to remain in proxy mode")
	}
}

func TestRequiredInitImageOmitsDefaultCommandAudit(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "practical"
	cfg.Network.Backend = "proxy"

	if got := RequiredInitImage(cfg); got != "" {
		t.Fatalf("RequiredInitImage() = %q, want empty string", got)
	}
}

func TestBuildArgvNoAuditNoEbpfOmitsInitImage(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "practical"
	cfg.Network.Backend = "proxy"
	cfg.Audit.Commands.Enabled = false

	plan := Plan{
		ProjectPath: "/tmp/project",
		StagingDir:  "/tmp/staging",
		Image:       "test:latest",
		Effective:   cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--init-image") {
		t.Fatalf("expected no init image without command audit or ebpf network, got:\n%s", joined)
	}
}

func TestBuildArgvEbpfNetworkName(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "strict"
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	cfg.Network.EBPF.NetworkName = "opencode-sandbox"

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--network opencode-sandbox") {
		t.Error("expected --network flag")
	}
}

func TestBuildArgvEbpfNoNetworkName(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "strict"
	cfg.Network.Backend = "ebpf"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	cfg.Network.EBPF.NetworkName = ""

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--network") {
		t.Error("should not add --network when networkName is empty")
	}
}

func TestBuildArgvNetworkOffUsesNone(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.Mode = "off"
	cfg.Network.Backend = "ebpf"
	cfg.Network.DefaultAction = "deny"
	cfg.Network.EBPF.InitImage = "opencode-sandbox-init:latest"
	cfg.Network.EBPF.NetworkName = "opencode-sandbox"

	plan := Plan{
		ProjectPath: "/tmp/project",
		StagingDir:  "/tmp/staging",
		Image:       "test:latest",
		Effective:   cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--network none") {
		t.Fatalf("expected network none for mode off, got:\n%s", joined)
	}
	if strings.Contains(joined, "--network opencode-sandbox") {
		t.Fatalf("mode off should not also add configured network name, got:\n%s", joined)
	}
}

func TestBuildArgvEventLogDir(t *testing.T) {
	cfg := config.Defaults()
	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		EventLogDir:  "/tmp/events",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "type=bind,source=/tmp/events,target=/sandbox/logs") {
		t.Error("expected event log dir mount")
	}
}

func TestBuildArgvDurableOpenCodeStateMounts(t *testing.T) {
	cfg := config.Defaults()
	plan := Plan{
		ProjectPath:       "/tmp/project",
		StagingDir:        "/tmp/staging",
		OpenCodeConfigDir: "/tmp/soc/config/opencode",
		OpenCodeDataDir:   "/tmp/soc/data/opencode",
		OpenCodeStateDir:  "/tmp/soc/state/opencode",
		Image:             "test:latest",
		Effective:         cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"type=bind,source=/tmp/soc/config/opencode,target=/sandbox/home/.config/opencode",
		"type=bind,source=/tmp/soc/data/opencode,target=/sandbox/home/.local/share/opencode",
		"type=bind,source=/tmp/soc/state/opencode,target=/sandbox/home/.local/state/opencode",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected argv to contain %q, got:\n%s", want, joined)
		}
	}
}

func TestBuildArgvExtraMounts(t *testing.T) {
	cfg := config.Defaults()
	cfg.Project.ExtraMounts = []string{"/tmp/cache:/cache:ro"}
	plan := Plan{
		ProjectPath: "/tmp/project",
		StagingDir:  "/tmp/staging",
		Image:       "test:latest",
		Effective:   cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "type=bind,source=/tmp/cache,target=/cache,readonly") {
		t.Fatalf("expected extra readonly mount, got:\n%s", joined)
	}
}

func TestBuildArgvLocalhostAccessEnabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.LocalhostAccess.Enabled = true
	cfg.Network.LocalhostAccess.IP = "203.0.113.113"
	cfg.Network.LocalhostAccess.Domain = "host.container.internal"

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--localhost 203.0.113.113") {
		t.Error("expected --localhost flag")
	}
	if !strings.Contains(joined, "OPENCODE_SANDBOX_HOST_DOMAIN=host.container.internal") {
		t.Error("expected OPENCODE_SANDBOX_HOST_DOMAIN env var")
	}
	if !strings.Contains(joined, "OPENCODE_SANDBOX_HOST_IP=203.0.113.113") {
		t.Error("expected OPENCODE_SANDBOX_HOST_IP env var")
	}
}

func TestBuildArgvLocalhostAccessDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Network.LocalhostAccess.Enabled = false

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--localhost") {
		t.Error("expected no --localhost flag when disabled")
	}
}

func TestBuildArgvReadonlyProject(t *testing.T) {
	cfg := config.Defaults()
	cfg.Project.ReadOnly = true

	plan := Plan{
		ProjectPath:  "/tmp/project",
		StagingDir:   "/tmp/staging",
		Image:        "test:latest",
		OpenCodeArgs: []string{},
		Effective:    cfg,
	}

	argv := BuildArgv(plan)
	for _, arg := range argv {
		if strings.Contains(arg, "source=/tmp/project") {
			if !strings.Contains(arg, ",readonly") {
				t.Error("expected readonly project mount")
			}
			return
		}
	}
	t.Error("project mount not found")
}

func TestRedactedCommand(t *testing.T) {
	argv := []string{
		"container", "run",
		"--env", "API_KEY=secret123",
		"--env", "TOKEN=abc",
		"--env", "MY_PASSWORD=pass",
		"--env", "AUTH_HEADER=bearer",
		"--env", "MY_SECRET=shh",
		"--env", "NORMAL=value",
	}
	redacted := RedactedCommand(argv)
	if strings.Contains(redacted, "secret123") {
		t.Error("API_KEY should be redacted")
	}
	if strings.Contains(redacted, "abc") {
		t.Error("TOKEN should be redacted")
	}
	if strings.Contains(redacted, "pass") {
		t.Error("PASSWORD should be redacted")
	}
	if strings.Contains(redacted, "bearer") {
		t.Error("AUTH should be redacted")
	}
	if strings.Contains(redacted, "shh") {
		t.Error("SECRET should be redacted")
	}
	if !strings.Contains(redacted, "NORMAL=value") {
		t.Error("NORMAL env should not be redacted")
	}
}
