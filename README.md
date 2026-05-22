# opencode-sandbox

A Go CLI wrapper that runs [OpenCode](https://opencode.ai) inside Apple's native `container` runtime on macOS, with configurable network policy and domain blocklisting.

## Quickstart

```bash
# Quick setup
curl -fsSL https://raw.githubusercontent.com/RabbITCybErSeC/opencode-sandbox/main/install.sh | bash
```

```bash
# Add the alias printed by the installer, then check your environment
sopencode doctor

# Create configs
sopencode init --global
sopencode init

# Fetch the published default image
sopencode image pull

# Run OpenCode in the sandbox
sopencode run .

# Or forward OpenCode args directly
sopencode --help
sopencode .
```

Manual development checkout:

```bash
git clone https://github.com/RabbITCybErSeC/opencode-sandbox.git
cd opencode-sandbox
git pull --ff-only
go build -o ./opencode-sandbox ./cmd/opencode-sandbox

# Check your environment
./opencode-sandbox doctor

# Create configs
./opencode-sandbox init --global
./opencode-sandbox init

# Fetch the published default image
./opencode-sandbox image pull

# Or build the image locally from source
./opencode-sandbox image build

# Run OpenCode in the sandbox
./opencode-sandbox run .

# Or forward OpenCode args directly
./opencode-sandbox --help
./opencode-sandbox .
```

For a fuller setup guide, including local CLI build, config paths, skill import, and strict eBPF setup, see [Quickstart](docs/quickstart.md).

## Network Modes

## Command Audit Logging

Command audit logging is opt-in while the custom init image path is experimental. When enabled, the init daemon attaches eBPF exec tracepoints inside the Apple container Linux VM and writes observed process exec events to the unified audit log:

```text
~/.local/state/opencode-sandbox/runs/<run-id>/audit-events.jsonl
```

The log includes full argv by default, so it may contain URLs, prompts, tokens, or other command-line secrets. If a run hangs at Apple container startup and the boot log mentions `/sbin/vminitd`, disable command audit and use practical proxy mode until the init image is rebuilt.

```yaml
audit:
  commands:
    enabled: true
    backend: ebpf
    failClosed: false
    logArgs: full
    excludeExecutables:
      - /usr/bin/true
```

Shell builtins that do not spawn a process are not separate exec events.

### Practical Proxy (Default)

Blocks configured domains via an HTTP CONNECT proxy inside the container. Easy to set up and suitable for daily development.

```yaml
network:
  mode: practical
  backend: proxy
  blocklist:
    - "*.segment.io"
```

### Host Network Access

Connect to MCP servers or other services running on your Mac from inside the sandbox:

```bash
# One-off
opencode-sandbox run . --allow-host-access

# Or persist in .opencode-sandbox.yaml
network:
  localhostAccess:
    enabled: true
```

One-time host setup (until reboot):
```bash
sudo container system dns create host.container.internal --localhost 203.0.113.113
```

Then use `http://host.container.internal:3000` in your `opencode.json` MCP config.

### Strict eBPF (Opt-In)

IPv4 cgroup/connect enforcement inside the Apple container Linux VM. Requires a strict init image.

```yaml
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  blocklist:
    - "*.telemetry.example.com"
  ebpf:
    initImage: ghcr.io/rabbitcybersec/opencode-sandbox-init:latest
    networkName: opencode-sandbox
```

Default-allow strict mode blocks configured exact-domain IPv4 destinations after resolution and allows unknown traffic. Default-deny blocks unknown IPv4 destinations and should be used only with an allowlist plan.

## Shell Alias

During active development, use a short wrapper alias:

```bash
alias sopencode='opencode-sandbox'
```

Then run:

```bash
sopencode run .
```

Direct OpenCode argument forwarding is supported, so you can alias `opencode` itself when you are ready:

```bash
alias opencode='opencode-sandbox'
```

## Documentation

- [Network Policy](docs/network-policy.md) — practical vs strict eBPF, configuration, event logs
- [Quickstart](docs/quickstart.md) — first install, config, image pull/build, and run
- [OpenCode Config Control](docs/quickstart.md#control-opencode-config-in-the-sandbox) — wrapper config paths and sandbox-managed OpenCode config/state
- [Security Model](docs/security-model.md) — threat model, isolation layers, limitations
- [Troubleshooting](docs/troubleshooting.md) — doctor checks, common issues, fixes
- [Implementation Spec](docs/implementation-spec.md) — architecture and design decisions

## Development

```bash
go test ./...
```

Published images can be pulled with:

```bash
opencode-sandbox image pull
opencode-sandbox image pull --strict-init
```

Local source builds remain supported:

```bash
opencode-sandbox image build
opencode-sandbox image build --strict-init
```

Live integration tests (require Apple container and macOS):

```bash
OPENCODE_SANDBOX_LIVE=1 go test -tags live ./internal/integration/...
```

## License

MIT
