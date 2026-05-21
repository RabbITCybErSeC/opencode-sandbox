# Quickstart

This guide gets opencode-sandbox from install to a first sandboxed OpenCode run.

## Prerequisites

- macOS with Apple's `container` CLI installed and available on `PATH`.
- Go installed for building the wrapper CLI.
- Network access to pull the published OpenCode container image, or to build it locally from source.

Check the environment first:

```bash
container system version
go version
```

## 1. Install the wrapper CLI

Install from GitHub:

```bash
curl -fsSL https://raw.githubusercontent.com/RabbITCybErSeC/opencode-sandbox/main/install.sh | bash
```

The installer clones or updates the repo, builds the wrapper CLI, pulls the published runtime image, and prints a ready-to-add shell alias. In an interactive shell it also asks whether to pull the optional strict eBPF init image.

```bash
alias sopencode="$HOME/.local/bin/opencode-sandbox"
```

After adding the alias to your shell profile, open a new shell or run the alias command directly.

## 2. Manual checkout and build

If you want a development checkout, clone the repo:

```bash
git clone https://github.com/RabbITCybErSeC/opencode-sandbox.git
cd opencode-sandbox
git pull --ff-only
```

From the repo root:

```bash
go build -o ./opencode-sandbox ./cmd/opencode-sandbox
```

Use the local binary directly while developing:

```bash
./opencode-sandbox help
./opencode-sandbox doctor
```

To make it available from any shell, move it somewhere on your `PATH` or create an alias:

```bash
alias sopencode='/path/to/opencode-sandbox'
```

## 3. Create configuration

Create global defaults under your host config directory:

```bash
sopencode init --global
```

Create a project config in the project you want to work on:

```bash
cd /path/to/project
sopencode init
```

The global config is stored under:

```bash
sopencode config path --global
```

The project config is stored in:

```bash
sopencode config path
```

Project config inherits global network policy by default, so global blocklist rules apply everywhere and project rules can add more specific entries.

## 4. Control OpenCode config in the sandbox

`opencode-sandbox` has its own wrapper config and also manages the OpenCode
config, data, and state that OpenCode sees inside the sandbox.

Wrapper config is stored in these locations:

```text
~/.config/opencode-sandbox/config.yaml
<project>/.opencode-sandbox.yaml
```

The sandbox-managed OpenCode directories are stored under:

```text
~/.config/opencode-sandbox/opencode/config
~/.config/opencode-sandbox/opencode/data
~/.config/opencode-sandbox/opencode/state
```

Use the config commands to find and inspect the active wrapper config:

```bash
sopencode config path --global
sopencode config path
sopencode config show
```

Control how OpenCode config is prepared for the sandbox with the `opencode:`
section in either the global or project wrapper config:

```yaml
opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false
```

- `mountHostConfig` copies selected host OpenCode config files into the
  sandbox-managed OpenCode config directory.
- `mountHostData` preserves OpenCode data and state between runs through
  wrapper-owned host directories.
- `generatedConfig` writes the managed OpenCode `opencode.json` overlay.
- `autoupdate` sets the `autoupdate` value in the generated overlay.

Edit `config.yaml` or `.opencode-sandbox.yaml` to control wrapper behavior. If
you need to manage OpenCode-facing files directly, use the sandbox-managed
OpenCode config directory above. When `generatedConfig: true`, the wrapper owns
`opencode.json`, so keep persistent manual edits in other OpenCode config files
or disable generated config deliberately.

## 5. Configure a blocklist

For daily use, start with practical proxy mode. Edit either the global config or the project config:

```yaml
network:
  mode: practical
  backend: proxy
  blocklist:
    - "*.segment.io"
    - "*.google-analytics.com"
```

Check how a domain will be handled:

```bash
sopencode policy test telemetry.segment.io
```

## 6. Fetch or build the OpenCode image

Fetch the published normal runtime image:

```bash
sopencode image pull
```

This fetches:

```text
ghcr.io/rabbitcybersec/opencode-sandbox:latest
```

Local source builds are still supported for development or custom OpenCode versions:

```bash
sopencode image build
sopencode image build --opencode-version <version>
```

## 7. Run OpenCode in a project

From a project folder:

```bash
sopencode run .
```

The wrapper also forwards OpenCode arguments directly, which makes aliases work:

```bash
sopencode --help
sopencode .
```

The selected project is mounted into the container at:

```text
/workspace
```

Wrapper state, OpenCode config/data/state, and event logs stay on the host. Network events are written under:

```text
~/.local/state/opencode-sandbox/runs/<run-id>/network-events.jsonl
```

Command audit events are opt-in while the custom init image path is experimental. When enabled, they are written under:

```text
~/.local/state/opencode-sandbox/runs/<run-id>/command-events.jsonl
```

Command audit uses eBPF exec tracing inside the container VM. It records process execs such as `curl`, `git`, `npm`, and helper binaries with full argv by default. Shell builtins that do not spawn a process are not separate events, and full argv can include secrets.

## 8. Import skills

Import skills globally:

```bash
sopencode skills import /path/to/skills --scope global
```

Import skills for only the current project:

```bash
sopencode skills import /path/to/skills --scope project
```

Use `--dry-run` first if you want to preview the import:

```bash
sopencode skills import /path/to/skills --scope project --dry-run
```

Imported global skills live under:

```text
<host config dir>/opencode-sandbox/skills
```

Imported project skills live under:

```text
<project>/.opencode-sandbox/skills
```

## Optional: Strict eBPF Mode

Strict eBPF mode is opt-in. It runs a policy daemon from a strict init image inside the Apple container Linux VM.

Fetch the strict init image:

```bash
sopencode image pull --strict-init
```

This fetches:

```text
ghcr.io/rabbitcybersec/opencode-sandbox-init:latest
```

You can also build it locally:

```bash
sopencode image build --strict-init
```

Then configure strict mode:

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

Run doctor after switching:

```bash
sopencode doctor
```

`defaultAction: allow` means unknown direct-IP traffic is allowed. Configured exact-domain IPv4 entries are blocked after resolution. Use `defaultAction: deny` only when you are ready for hard isolation and have an allowlist plan.

## Useful Commands

```bash
sopencode help
sopencode doctor
sopencode config show
sopencode policy test example.com
sopencode run .
sopencode run . --dry-run
```

## Optional Shell Alias

During active development, keep the wrapper command explicit:

```bash
alias sopencode='opencode-sandbox'
sopencode run .
```

Direct OpenCode argument forwarding is enabled, so you can alias `opencode` itself:

```bash
alias opencode='opencode-sandbox'
```

Live strict-mode checks are gated behind a build tag and environment variable:

```bash
OPENCODE_SANDBOX_LIVE=1 go test -tags live ./internal/integration/...
```
