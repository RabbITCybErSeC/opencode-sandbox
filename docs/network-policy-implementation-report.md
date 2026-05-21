# Implementation Report: `--allow-host-access` Feature

## Summary

Added support for connecting to services running on the macOS host (e.g., MCP servers on `localhost:3000`) from inside the Apple `container` sandbox.

## What Changed

### 1. Config Schema (`internal/config/config.go`)

Added `LocalhostAccess` struct with three fields:
- `enabled` (bool): whether host access is enabled
- `ip` (string): the IPv4 address assigned to the host domain inside the container
- `domain` (string): the DNS domain name used to reach the host

Added to both `Network` (YAML-parsing) and `EffectiveNetwork` (runtime-resolved).

### 2. Defaults (`internal/config/defaults.go`)

```go
LocalhostAccess: EffectiveLocalhostAccess{
    Enabled: false,
    IP:      "203.0.113.113",
    Domain:  "host.container.internal",
}
```

### 3. Merge Logic (`internal/config/load.go`)

Added `applyLocalhostAccess()` function that merges YAML overrides into the effective config. Follows the same pointer-backed pattern as other config sections.

### 4. Validation (`internal/config/validate.go`)

When `localhostAccess.enabled` is true:
- `ip` must be non-empty
- `ip` must be a valid IPv4 address (uses `net.ParseIP`)
- `domain` must be non-empty

### 5. Container Command Builder (`internal/containercmd/builder.go`)

When `LocalhostAccess.Enabled` is true, appends to `container run` argv:
- `--env OPENCODE_SANDBOX_HOST_DOMAIN=host.container.internal`
- `--env OPENCODE_SANDBOX_HOST_IP=203.0.113.113`

Host DNS itself is configured outside `container run` with:

```bash
sudo container system dns create host.container.internal --localhost 203.0.113.113
```

### 6. CLI Flag (`internal/cli/run.go`, `internal/cli/run_plan.go`)

Added `--allow-host-access` flag parsing:
- Strips the flag from args before forwarding to OpenCode
- Sets `plan.AllowHostAccess = true`
- CLI flag overrides config: if `--allow-host-access` is passed, `base.Network.LocalhostAccess.Enabled = true` regardless of YAML config

### 7. Doctor Check (`internal/doctor/checks.go`)

New check `host.dns`:
- **Skip** if `localhostAccess` is not enabled
- **Warn** if not on macOS
- **Warn** if the domain is not configured on the host, printing the exact setup command:
  ```bash
  sudo container system dns create host.container.internal --localhost 203.0.113.113
  ```
- **Pass** if the domain resolves

### 8. Init Templates (`internal/cli/init.go`)

Both global and project config YAML templates now include a commented `localhostAccess` block:
```yaml
# Allow the container to reach services on the macOS host (e.g., MCP servers).
# Requires: sudo container system dns create host.container.internal --localhost 203.0.113.113
localhostAccess:
  enabled: false
  ip: 203.0.113.113
  domain: host.container.internal
```

### 9. Tests (`internal/containercmd/builder_test.go`, `internal/cli/init_config_test.go`)

- `TestBuildArgvLocalhostAccessEnabled`: verifies no invalid `container run --localhost` flag is present and both env vars are present
- `TestBuildArgvLocalhostAccessDisabled`: verifies no `--localhost` flag when disabled
- `TestInitProjectCreatesConfig`: verifies generated config contains `localhostAccess:`

### 10. Documentation

- `docs/network-policy.md`: Added full "Host Network Access" section with CLI, config, host setup, and OpenCode MCP examples
- `README.md`: Added quick `### Host Network Access` section between Practical Proxy and Strict eBPF

## Usage

### One-off CLI
```bash
opencode-sandbox run . --allow-host-access
```

### Persistent project config (`.opencode-sandbox.yaml`)
```yaml
network:
  localhostAccess:
    enabled: true
```

### Host-side setup (one-time until reboot)
```bash
sudo container system dns create host.container.internal --localhost 203.0.113.113
```

### OpenCode MCP config (`opencode.json`)
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

## Files Modified

| File | Lines | Description |
|---|---|---|
| `internal/config/config.go` | +54/-35 | Added `LocalhostAccess` and `EffectiveLocalhostAccess` structs |
| `internal/config/defaults.go` | +5 | Default `localhostAccess` values |
| `internal/config/load.go` | +17 | `applyLocalhostAccess()` merge function |
| `internal/config/validate.go` | +12 | IP and domain validation |
| `internal/containercmd/builder.go` | +5 | Host access env vars |
| `internal/containercmd/builder_test.go` | +48 | Two new test cases |
| `internal/cli/run.go` | +13/-2 | `--allow-host-access` flag parsing and override |
| `internal/cli/run_plan.go` | +5/-2 | `AllowHostAccess` field on `RunPlan` |
| `internal/cli/init.go` | +12 | `localhostAccess` in global and project templates |
| `internal/cli/init_config_test.go` | +3 | Assert `localhostAccess` in generated config |
| `internal/doctor/checks.go` | +46 | `checkHostDNS()` function |
| `docs/network-policy.md` | +54 | Full host access documentation |
| `README.md` | +21 | Quick example |

## Validation Checklist

- [ ] `go test ./...` passes
- [ ] `go build ./cmd/opencode-sandbox` succeeds
- [ ] `opencode-sandbox init` generates config with `localhostAccess` section
- [ ] `opencode-sandbox init --global` generates config with `localhostAccess` section
- [ ] `opencode-sandbox run . --allow-host-access --print-command` shows host access env vars and no `--localhost` run flag
- [ ] `opencode-sandbox doctor` shows `host.dns` check when `localhostAccess.enabled: true`
- [ ] Config validation rejects invalid IP or missing domain when enabled
