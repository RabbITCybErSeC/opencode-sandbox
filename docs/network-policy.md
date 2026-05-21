# Network Policy

opencode-sandbox supports two network backends:

- **practical** (default): proxy-based blocklisting
- **strict eBPF** (opt-in): cgroup/connect enforcement inside the Apple container VM

It also supports **host network access** so the container can reach services running on the macOS host (e.g., local MCP servers).

## Practical Proxy Mode

Practical mode is the default. It runs a small HTTP CONNECT proxy inside the container that blocks configured domains. This mode is easy to set up and works on any system with Apple container.

```yaml
network:
  mode: practical
  backend: proxy
  blocklist:
    - "*.segment.io"
    - "*.google-analytics.com"
```

Proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`) are injected automatically.

### Limitations

- Direct IP connections bypass domain policy.
- Applications that ignore proxy env vars may bypass enforcement.
- Encrypted DNS (DoH) can bypass DNS-level blocking.

Practical mode is a developer guardrail, not a hard security boundary.

## Host Network Access

By default, `localhost` inside the container refers to the container itself, not the macOS host. To let OpenCode connect to an MCP server or other service running on your Mac, enable `localhostAccess`.

### One-off CLI flag

```bash
opencode-sandbox run . --allow-host-access
```

### Persistent project config (`.opencode-sandbox.yaml`)

```yaml
network:
  localhostAccess:
    enabled: true
    ip: 203.0.113.113
    domain: host.container.internal
```

### Host-side setup (one-time until reboot)

Apple container requires a DNS domain to be registered on the host before the container can resolve it. Run once per reboot:

```bash
sudo container system dns create host.container.internal --localhost 203.0.113.113
```

> **Warning:** This feature uses macOS packet filter rules and disables iCloud Private Relay while active. The rule is removed on system restart.

### Usage in OpenCode config

Once enabled, configure your MCP server in `opencode.json` using the host domain:

```json
{
  "mcp": {
    "my-local-server": {
      "type": "remote",
      "url": "http://host.container.internal:3000"
    }
  }
}
```

### Inside the container

The following environment variables are available when host access is enabled:

- `OPENCODE_SANDBOX_HOST_DOMAIN=host.container.internal`
- `OPENCODE_SANDBOX_HOST_IP=203.0.113.113`

## Strict eBPF Mode

Strict mode moves enforcement into the Apple container Linux VM using eBPF. It is opt-in and requires:

- macOS with Apple container
- A strict init image (`opencode-sandbox-init`)
- Optional custom container network

### How It Works

The eBPF program runs **inside the Linux VM** used by Apple `container`, not directly on the macOS host. At VM boot, the init image starts a policy daemon that:

1. Reads the runtime policy bundle.
2. Attaches an IPv4 cgroup/connect eBPF hook and fails closed when attach fails.
3. Resolves exact domain rules to IPv4 map entries with TTL-aware refresh.
4. Attaches command audit exec tracepoints when `audit.commands.enabled` is true.
5. Writes daemon lifecycle, resolver-derived, and command audit events to durable JSONL logs.

Current strict mode is intentionally minimal. It enforces exact IPv4 destinations through eBPF maps. Wildcard domains require future DNS interception to become complete strict enforcement, and per-connection eBPF event export is still a follow-up item.

### Configuration

Global config:

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
    eventLog: ~/.local/state/opencode-sandbox/runs
    mirrorProjectEvents: false
```

Project config can add project-specific blocks:

```yaml
network:
  mode: strict
  backend: ebpf
  blocklist:
    - "*.client-tracking.example.com"
```

### defaultAction

- **`allow`** (default for strict): unknown direct-IP traffic is allowed.
- **`deny`**: unknown direct-IP traffic is blocked.

Default-allow strict mode gives you monitoring plus configured blocking without breaking provider traffic.

### Mode: off

Network-off mode requires `backend: ebpf` and `defaultAction: deny`. Proxy backend is rejected for `mode: off` because it cannot block direct-IP bypasses.

```yaml
network:
  mode: off
  backend: ebpf
  defaultAction: deny
  ebpf:
    initImage: ghcr.io/rabbitcybersec/opencode-sandbox-init:latest
```

## Event Logs

Strict mode writes JSONL event logs. Practical proxy mode writes proxy block logs.

- **Host state**: `~/.local/state/opencode-sandbox/runs/<run-id>/network-events.jsonl`
- **Project mirror** (optional): `<project>/.opencode-sandbox/network-events.jsonl`

Strict event fields include timestamp, run ID, project, backend, hook, protocol, destination IP/port, decision, reason, and matched rule when available. Events never include URLs, query strings, headers, request bodies, or secrets.

Command audit events are enabled by default and written beside network events.

- **Host state**: `~/.local/state/opencode-sandbox/runs/<run-id>/command-events.jsonl`
- **Project mirror** (optional): `<project>/.opencode-sandbox/command-events.jsonl`

Command audit records `execve` and `execveat` process launches inside the container VM, including tools such as `curl`, `git`, `npm`, shell-spawned commands, and helper binaries. It logs full argv by default, which can include secrets passed on the command line. Shell builtins that do not spawn a process are not separate events.

## Switching Backends

Use `opencode-sandbox doctor` to check whether strict eBPF mode is supported on your machine. If eBPF is unsupported, the tool will recommend practical proxy mode.

```bash
opencode-sandbox doctor
opencode-sandbox doctor --json
```

## Domain-to-IP Limitations

Domain blocklists are resolved to IP addresses for eBPF enforcement. This has inherent limitations:

- Domains that resolve to shared or changing IPs may affect unrelated traffic.
- Wildcard domains (`*.example.com`) require runtime DNS interception; exact domains are pre-resolved.
- Shared CDNs or load balancers may cause over-blocking or under-blocking.

TTL-aware refresh mitigates some of this for exact domains, but it is not perfect. Treat wildcard-domain strict enforcement and per-connection eBPF event export as follow-up work until live tests prove them.
