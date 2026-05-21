# Security Model

opencode-sandbox is a developer isolation wrapper around OpenCode. It is designed to make the secure path easy, not to provide a formally verified sandbox.

## Threat Model

### What opencode-sandbox Protects Against

- **Accidental data exfiltration**: domain blocklists prevent common analytics, telemetry, and tracking endpoints.
- **Ambient network access**: strict eBPF mode can limit IPv4 outbound connections inside the container VM.
- **Credential leakage**: OpenCode config/data/state live in wrapper-owned host directories instead of mounting the full host home.

### What It Does Not Protect Against

- **Malicious kernel-level workloads**: the wrapper runs inside Apple's container VM; a determined attacker with kernel access inside the VM can bypass eBPF filters.
- **Application-level bypasses**: applications that ignore proxy env vars or use hardcoded IPs may bypass practical mode.
- **Side channels**: timing, power, or acoustic side channels are not addressed.

## Isolation Layers

1. **Apple container VM**: each container runs in a lightweight VM with its own Linux kernel.
2. **eBPF cgroup hooks** (strict mode): IPv4 cgroup/connect filtering before connections leave the VM.
3. **eBPF exec tracepoints**: command audit records process execs inside the VM.
4. **Read-only mounts**: host config and skills are mounted read-only.
5. **Durable wrapper-owned OpenCode state**: config/data/state are mounted from host-side opencode-sandbox directories.

## eBPF Strict Mode Security Properties

### Where eBPF Runs

The eBPF program runs **inside the Apple container Linux VM**, not on the macOS host. This means:

- It observes traffic from the OpenCode process namespace inside the VM.
- It cannot filter traffic from other macOS processes.
- It requires the Apple container runtime and a compatible Linux kernel.

### Default-Allow vs Default-Deny

- **defaultAction=allow** (recommended for daily use): configured exact-domain IPv4 destinations are blocked after resolution; unknown traffic is allowed.
- **defaultAction=deny**: unknown IPv4 destinations are blocked by the eBPF hook. Use this only with an allowlist plan.

Strict mode currently does not claim complete wildcard-domain enforcement or complete IPv6 coverage. Those require additional DNS interception/map coverage and live validation before they should be treated as hard isolation.

### Event Log Privacy

Network event logs are designed to be privacy-preserving:

- **Logged**: timestamp, IP, port, process name, decision, reason, matched rule.
- **Not logged**: URLs, query strings, HTTP headers, request bodies, tokens, secrets.

Command audit logs intentionally prioritize forensic fidelity:

- **Logged**: timestamp, PID, parent PID, UID/GID, executable, working directory, argc, argv, decision, reason.
- **Not logged as separate events**: shell builtins that do not call `execve` or `execveat`.
- **Sensitive data warning**: full argv logging is enabled by default and can include tokens, URLs, prompts, auth headers, or other secrets passed on the command line.

## Failure Modes

### Fail-Closed

When `failClosed: true` (the default):

- If strict network eBPF cannot attach hooks, OpenCode does not start.
- If the policy proxy fails to start in practical mode, OpenCode does not start.
- If the policy bundle cannot be generated, the run aborts.

Command audit uses `audit.commands.failClosed: false` by default. If the command audit eBPF tracepoints cannot attach, the daemon warns and lets OpenCode continue.

### Fallback

If strict eBPF mode is unsupported on your system:

1. Run `opencode-sandbox doctor` to see what's missing.
2. Switch to practical proxy mode by setting `network.mode: practical`.
3. Practical mode blocks normal DNS/proxy traffic but cannot guarantee direct-IP blocking.

## Configuration Examples

### Global Blocklist

`~/.config/opencode-sandbox/config.yaml`:

```yaml
version: 1
network:
  mode: practical
  blocklist:
    - "*.segment.io"
    - "*.google-analytics.com"
```

### Project-Specific Strict Mode

`<project>/.opencode-sandbox.yaml`:

```yaml
version: 1
network:
  mode: strict
  backend: ebpf
  defaultAction: allow
  blocklist:
    - "*.client-telemetry.example.com"
  ebpf:
    initImage: ghcr.io/rabbitcybersec/opencode-sandbox-init:latest
    mirrorProjectEvents: true
```

### Command Audit

Command audit is enabled by default and works with practical proxy mode or strict eBPF networking:

```yaml
audit:
  commands:
    enabled: true
    backend: ebpf
    failClosed: false
    logArgs: full
    maxArgs: 64
    maxArgBytes: 16384
    includeExecutables: []
    excludeExecutables: []
    includeCwd: []
    excludeCwd: []
    mirrorProjectEvents: false
```

Host log path:

```text
~/.local/state/opencode-sandbox/runs/<run-id>/command-events.jsonl
```

## Best Practices

- Start with practical mode and a small blocklist.
- Use `opencode-sandbox policy test <domain>` to verify rules.
- Review event logs periodically to understand traffic patterns.
- Keep the strict init image up to date.
- Do not mount the full host home directory.
