# Troubleshooting

## Doctor Checks

Run `opencode-sandbox doctor` to diagnose environment issues.

```bash
opencode-sandbox doctor
opencode-sandbox doctor --json
```

### Common Doctor Outputs

#### `container.binary` fail

Apple container is not installed or not in PATH.

**Fix**: Install Apple's `container` tool and ensure it is in your PATH.

#### `ebpf.init-image` warn

The configured strict init image is not found locally. Practical proxy mode does not require an init image unless command audit is explicitly enabled.

**Fix**: Pull the published init image, or build it locally from source:

```bash
opencode-sandbox image pull --strict-init
opencode-sandbox image build --strict-init
```

If you do not need strict eBPF or command audit, disable command audit:

```yaml
audit:
  commands:
    enabled: false
```

#### `ebpf.network-name` warn

The configured custom network does not exist.

**Fix**: Create the network:

```bash
container network create opencode-sandbox
```

#### `ebpf.support` fail

eBPF strict mode is not supported on this system.

**Fix**: Switch to practical proxy mode in your config:

```yaml
network:
  mode: practical
  backend: proxy
```

## Strict Mode Issues

### OpenCode does not start

If `failClosed: true` and the eBPF daemon cannot attach hooks, the run aborts.

**Check the init image logs**:

```bash
container logs <container-name>
```

Common causes:
- cgroup2 is not available in the Apple container VM.
- The init image is missing or corrupted.
- The policy bundle is malformed.

If the boot log ends with `Requested init /sbin/vminitd failed (error -2)`, the init image is not bootable as an Apple container init image. Disable command audit or strict eBPF for the project and retry in practical proxy mode.

### Blocked provider traffic

If an OpenCode provider is blocked unexpectedly:

1. Check `policy test`:

```bash
opencode-sandbox policy test api.openai.com
```

2. Add the provider to your allowlist if needed:

```yaml
network:
  allowlist:
    - "api.openai.com"
```

3. Review event logs to see the exact decision:

```bash
cat ~/.local/state/opencode-sandbox/runs/<latest-run>/network-events.jsonl
```

### Event logs are empty

Event logs are written by the eBPF daemon inside the init image. If logs are missing:

- Verify the init image is running.
- Check that the event log directory is mounted at `/sandbox/logs`.
- Ensure `mirrorProjectEvents: true` is set if you expect project-local mirrors.

## Practical Mode Issues

### Proxy not blocking traffic

Practical mode relies on proxy env vars. Some applications ignore them.

**Symptoms**: Blocked domains are still reachable.

**Fix**: Switch to strict eBPF mode for kernel-level enforcement, or configure the application to respect proxy settings.

### DNS bypass

Encrypted DNS (DoH/DNS-over-TLS) can bypass the local DNS resolver.

**Fix**: Use strict eBPF mode, which enforces at the IP layer regardless of DNS method.

## General Issues

### Config not found

Project config is discovered by walking up from the project path. Global config lives at `~/.config/opencode-sandbox/config.yaml`.

### Image build fails

For normal installs, prefer pulling the published image:

```bash
opencode-sandbox image pull
```

For local source builds, use a development checkout and pass the source explicitly. The installer downloads a prebuilt wrapper binary and no longer creates `~/.local/share/opencode-sandbox-src`.

```bash
opencode-sandbox image build --context /path/to/opencode-sandbox
opencode-sandbox image build --file /path/to/opencode-sandbox/Containerfile
```

For the strict init image:

```bash
opencode-sandbox image pull --strict-init
opencode-sandbox image build --strict-init
```

### Permission denied on project mount

Writes from the Linux container to a macOS bind mount may have ownership quirks.

**Fix**: Apple container handles most ownership automatically. If issues persist, check the `--uid` and `--gid` flags.
