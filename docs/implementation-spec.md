# Implementation Spec: Sandboxed OpenCode on macOS Native Containers

Status: Draft for implementation handoff
Date: 2026-05-18
Repository: `opencode-sandbox`

## 1. Purpose

Build a small Go command-line wrapper that runs OpenCode inside Apple's native
`container` runtime on macOS. The wrapper should make the secure path easy:
users choose a project folder, the wrapper mounts that project into the
container, OpenCode runs with the user's existing OpenCode credentials/config,
and the wrapper applies a configurable network policy with domain blocklisting.

This document is intentionally implementation-oriented. It is written so another
agent can pick up the work without re-deciding the product shape.

## 2. Source References

Implementation agents should re-check these before coding because both Apple
`container` and OpenCode are active projects.

- Apple `container` docs index: https://github.com/apple/container/tree/main/docs
- Apple command reference: https://raw.githubusercontent.com/apple/container/refs/heads/main/docs/command-reference.md
- Apple how-to: https://raw.githubusercontent.com/apple/container/refs/heads/main/docs/how-to.md
- Apple technical overview: https://raw.githubusercontent.com/apple/container/refs/heads/main/docs/technical-overview.md
- OpenCode docs: https://opencode.ai/docs/
- OpenCode config: https://opencode.ai/docs/config
- OpenCode skills: https://opencode.ai/docs/skills

Key facts from the references that shape this design:

- Apple `container` runs each container in a lightweight VM, not in a shared
  Docker-style Linux VM. This gives stronger isolation and supports mounting
  only the host data needed for a specific run.
- `container run` supports `--volume`, `--mount`, `--read-only`, `--tmpfs`,
  `--dns`, `--network`, `--init`, `--init-image`, `--cpus`, `--memory`, `--uid`,
  `--gid`, `--workdir`, `--tty`, and `--interactive`.
- `--init-image` can be used for boot-time VM customization, including
  VM-level daemons and eBPF-style filtering. Use this for strict egress mode.
- `container build` can build OCI images from a Dockerfile or Containerfile.
- `container system start` is needed before use, and the user may need the
  recommended Linux kernel installed by Apple's tooling.
- OpenCode global config lives at `~/.config/opencode/opencode.json`.
- OpenCode project config is `opencode.json` in the project root.
- OpenCode TUI config may live at `~/.config/opencode/tui.json` or project
  `tui.json`.
- OpenCode skills are directories containing `SKILL.md`; OpenCode searches:
  `.opencode/skills`, `~/.config/opencode/skills`, `.claude/skills`,
  `~/.claude/skills`, `.agents/skills`, and `~/.agents/skills`.
- OpenCode supports `permission`, `enabled_providers`, `disabled_providers`,
  `instructions`, `mcp`, `plugin`, `lsp`, `formatter`, and `autoupdate`
  settings in config.

## 3. Goals

1. Run OpenCode in an isolated Linux container on macOS using Apple `container`.
2. Mount exactly one project folder into the container by default.
3. Preserve developer ergonomics:
   - Existing OpenCode auth/config should work.
   - First run should be discoverable through `doctor`, `init`, and `run`.
   - Users should not have to understand Apple `container` flags to use it.
4. Support domain blocklisting:
   - A practical default mode that is easy to set up.
   - A strict mode that provides stronger egress control when requested.
5. Make skills easy to import:
   - Explicit import/sync workflow.
   - No ambient import of all global skills.
6. Keep the wrapper auditable:
   - Deterministic command construction.
   - Clear generated files.
   - Clear policy explanation.
   - Testable modules.

## 4. Non-Goals for v1

- No GUI or macOS app.
- No support for Docker Desktop as a runtime.
- No automatic mounting of the user's full home directory.
- No multi-project workspace mount in the default path.
- No remote container orchestration.
- No attempt to perfectly sandbox malicious kernel-level workloads inside the
  Linux guest. This is a developer isolation tool around OpenCode, not a
  formally verified sandbox.
- No automatic migration of every OpenCode plugin/MCP server into the sandbox.
  Plugins/MCP are allowed only if explicitly configured.

## 5. Product Shape

The first deliverable is a Go CLI:

```text
opencode-sandbox <command> [options]
```

Recommended binary aliases:

- Primary: `opencode-sandbox`
- Optional user-defined shell alias, documented in README:
  `alias opencode='opencode-sandbox'`
- Optional short alias: `sopencode`

Use Go because:

- A single binary is easy to distribute.
- It is good at subprocess control and deterministic argv construction.
- It is a better fit for wrapping macOS tooling than a shell script.
- Tests can cover command generation, config validation, and policy behavior
  without needing the real Apple runtime.

## 6. User Experience

### 6.1 First-Time Setup

Expected flow:

```text
opencode-sandbox doctor
opencode-sandbox init --global
opencode-sandbox init
opencode-sandbox skills import ~/.config/opencode/skills
opencode-sandbox run .
```

The CLI should fail with actionable messages. For example:

- If `container` is missing: tell the user to install Apple's `container`.
- If the service is stopped: suggest `container system start`.
- If the image is missing: suggest `opencode-sandbox image build` or offer
  to build in the future. For v1, implement a clear command and no hidden build.
- If the project path does not exist: show the resolved absolute path.
- If strict mode is configured but unsupported: explain what feature is missing
  and how to switch to practical mode.

### 6.2 Normal Run

```text
opencode-sandbox run .
opencode-sandbox .
cd /path/to/project && opencode-sandbox
```

Expected behavior:

- If a project path is supplied, resolve it to an absolute host path.
- If no project path is supplied, use the current working directory.
- Validate it is a directory.
- Mount it at `/workspace`.
- Set working directory to `/workspace`.
- Run OpenCode interactively with TTY and stdin.
- Mount or stage OpenCode host config/auth according to the config policy.
- Apply network policy.
- Remove the container after exit.
- Propagate OpenCode's exit code.

The wrapper should be optimized for aliasing. If a user aliases `opencode` to
`opencode-sandbox`, normal OpenCode muscle memory should continue to work:

```text
opencode
opencode --help
opencode run "summarize this repo"
```

In these examples, the wrapper mounts the current directory as `/workspace` and
forwards `--help` or `run "summarize this repo"` directly to OpenCode.

### 6.3 Passing OpenCode Arguments

The primary UX is direct OpenCode forwarding. Anything that is not a wrapper
command or wrapper flag should be treated as OpenCode arguments:

```text
opencode-sandbox --help
opencode-sandbox run "summarize this repo"
opencode-sandbox --model anthropic/claude-sonnet-4-5
```

`--` should still be supported as an escape hatch, but it should not be required
for normal use:

```text
opencode-sandbox -- --help
opencode-sandbox . -- run "summarize this repo"
```

Argument parsing rules:

- Wrapper subcommands are reserved words: `doctor`, `init`, `run`, `skills`,
  `policy`, `image`, `config`, and `help`.
- Root-level `--help` and `-h` belong to OpenCode, not the wrapper. Wrapper help
  is available through `opencode-sandbox help` and subcommand help such as
  `opencode-sandbox help init`.
- If the first positional arg is an existing directory, treat it as the project
  path and forward the remaining args to OpenCode.
- If no project path is supplied, use the current working directory.
- If the first arg is not a wrapper subcommand and not an existing directory,
  treat all args as OpenCode args.
- Always prepend `opencode` inside the container. Users should not need to type
  `opencode-sandbox -- opencode`.
- Keep these rules documented and covered by tests because they are core UX.

### 6.4 Stateful Host Configuration

The wrapper should be stateful. It should keep durable wrapper state on the host
under the user's config directory rather than forcing every project to recreate
policy, image, and imported-skill metadata.

Host state locations:

- Global config: `~/.config/opencode-sandbox/config.yaml`
- Global imported skills: `~/.config/opencode-sandbox/skills`
- Global policy files/cache: `~/.config/opencode-sandbox/policy`
- Runtime cache/state, if needed: `~/.local/state/opencode-sandbox` or the
  platform-appropriate state directory
- Project config: `<project>/.opencode-sandbox.yaml`
- Project imported skills: `<project>/.opencode-sandbox/skills`

The global config supplies default network policy, image, resource, and OpenCode
mount behavior. Project config overrides or extends those defaults.

## 7. CLI Contract

### 7.1 `doctor`

Purpose: non-mutating environment check.

Command:

```text
opencode-sandbox doctor [--json]
```

Checks:

- OS is macOS.
- Architecture is Apple silicon arm64.
- macOS version is suitable for the chosen feature set:
  - macOS 26+ recommended for custom networks.
  - macOS 15 may have limitations; print warnings, not hard failures, unless
    configured features require macOS 26.
- `container` binary exists.
- `container system version` works.
- `container system status` or equivalent command works. If Apple changes the
  command, implement a compatibility layer after checking current docs.
- Default image exists locally.
- OpenCode host config paths exist or are absent with clear explanation.
- Strict mode support check:
  - custom init image exists if configured
  - required kernel/init capabilities are present if detectable

JSON output shape:

```json
{
  "ok": true,
  "checks": [
    {
      "id": "container.binary",
      "status": "pass",
      "message": "container found at /usr/local/bin/container"
    }
  ]
}
```

Statuses: `pass`, `warn`, `fail`, `skip`.

### 7.2 `init`

Purpose: create project config or global config.

Command:

```text
opencode-sandbox init [--project <path>] [--global] [--force]
```

Behavior:

- By default, writes `.opencode-sandbox.yaml` into the project root.
- With `--global`, writes `~/.config/opencode-sandbox/config.yaml`.
- Refuses to overwrite unless `--force` is set.
- Detects existing `opencode.json`, `.opencode/skills`, `.agents/skills`, and
  `.claude/skills` to produce useful comments in the generated config.

Default project config to generate:

```yaml
version: 1

image:
  name: ghcr.io/rabbitcybersec/opencode-sandbox:latest
  autoBuild: false

project:
  target: /workspace
  readonly: false

opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false

skills:
  importedDir: .opencode-sandbox/skills
  include:
    - "*"
  exclude: []

network:
  # Inherit global network defaults and add project-specific rules here.
  inheritGlobal: true
  mode: practical
  blocklist: []
  allowlist: []

resources:
  cpus: 4
  memory: 4g

container:
  namePrefix: opencode-sandbox
  remove: true
```

Default global config to generate:

```yaml
version: 1

image:
  name: ghcr.io/rabbitcybersec/opencode-sandbox:latest
  autoBuild: false

opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false

skills:
  importedDir: ~/.config/opencode-sandbox/skills
  include:
    - "*"
  exclude: []

network:
  mode: practical
  blocklist: []
  allowlist: []
  proxyPort: 18080
  dnsPort: 15353
  failClosed: true

resources:
  cpus: 4
  memory: 4g
```

### 7.3 `run`

Purpose: run OpenCode in the sandbox.

Command:

```text
opencode-sandbox [project] [wrapper flags] [opencode args...]
opencode-sandbox run [project] [wrapper flags] [opencode args...]
```

Required behavior:

- Load config from:
  1. `--config`
  2. global `~/.config/opencode-sandbox/config.yaml`
  3. nearest `.opencode-sandbox.yaml` from project path upward
  4. defaults if no config exists
- Validate config before launching.
- Merge global and project config before launching.
- Render effective config with `--dry-run`.
- Render exact `container run` argv with `--print-command`.
- Never execute through shell string concatenation. Always use argv arrays.
- Use `exec.CommandContext` or equivalent Go process API.
- Forward stdin/stdout/stderr.
- Preserve terminal behavior for TUI mode.
- Return OpenCode's exit code.

Important flags:

- `--dry-run`: validate and print effective plan, do not run.
- `--print-command`: print sanitized `container run` command.
- `--keep`: do not remove container after exit; useful for debugging.
- `--debug`: wrapper logs plus `container --debug` where appropriate.

Global/project merge behavior:

- Global config provides defaults.
- Project config overrides scalar fields such as `network.mode`, `resources`,
  image name, and OpenCode mount behavior when present.
- Project `network.blocklist` and `network.allowlist` are appended to global
  rules when `network.inheritGlobal: true`.
- If `network.inheritGlobal: false`, project network lists replace global
  network lists for that project.
- The effective config should be visible with `--dry-run`.

### 7.4 `skills import`

Purpose: copy selected skills into wrapper-controlled global or project state.

Command:

```text
opencode-sandbox skills import <source> [--scope global|project] [--project <path>] [--include <glob>] [--exclude <glob>] [--name-prefix <prefix>] [--dry-run]
```

Supported sources:

- A directory that contains skill subdirectories.
- A single skill directory containing `SKILL.md`.
- Later: Git repository URL, but not required for v1.

Behavior:

- Validate each skill directory has `SKILL.md`.
- Parse frontmatter enough to read `name` and `description`.
- Reject duplicate names by default.
- Support `--name-prefix` to intentionally disambiguate imported skills.
- Default to global imports at
  `~/.config/opencode-sandbox/skills/<skill-name>/SKILL.md`.
- With `--scope project`, copy into
  `<project>/.opencode-sandbox/skills/<skill-name>/SKILL.md`.
- Copy associated files in that skill directory.
- Preserve relative files referenced by the skill.
- Do not follow symlinks outside the source tree unless explicitly allowed in a
  future version.
- Write/update the matching global or project `skills-manifest.json`.

Manifest shape:

```json
{
  "version": 1,
  "importedAt": "2026-05-18T12:00:00Z",
  "skills": [
    {
      "name": "example",
      "source": "/Users/name/.config/opencode/skills/example",
      "target": "~/.config/opencode-sandbox/skills/example",
      "scope": "global",
      "sha256": "..."
    }
  ]
}
```

### 7.5 `skills list`

Purpose: show available imported and project-native skills.

Command:

```text
opencode-sandbox skills list [--json]
```

Include:

- global imported skills under `~/.config/opencode-sandbox/skills`
- project imported skills under `.opencode-sandbox/skills`
- project skills under `.opencode/skills`, `.agents/skills`, `.claude/skills`
- OpenCode/Claude/Agents global skills only as "available to import", not
  active by default

### 7.6 `policy test`

Purpose: explain network policy decisions.

Command:

```text
opencode-sandbox policy test <domain-or-url> [--project <path>] [--json]
```

Example output:

```text
blocked: api.segment.io
matched rule: *.segment.io
mode: practical
note: practical mode blocks normal DNS/proxy traffic but cannot guarantee direct-IP blocking
```

When `--project` is supplied, test against the merged global/project policy.
Without `--project`, test against global policy only.

### 7.7 `config`

Purpose: inspect merged configuration and help users debug global/project
settings.

Commands:

```text
opencode-sandbox config path [--global] [--project <path>]
opencode-sandbox config show [--project <path>] [--json]
```

Behavior:

- `config path --global` prints `~/.config/opencode-sandbox/config.yaml`.
- `config path --project <path>` prints the discovered project config path.
- `config show` prints the effective merged config with secrets redacted.

### 7.8 `image build`

Purpose: build the OpenCode image.

Command:

```text
opencode-sandbox image build [--tag ghcr.io/rabbitcybersec/opencode-sandbox:latest] [--opencode-version <version>]
```

Behavior:

- Runs `container build`.
- Uses the repo-provided Containerfile/Dockerfile.
- Pins OpenCode version if provided.
- Avoids secrets at build time.
- Emits a clear next command on success.

## 8. Config Schema

Implement config as a versioned YAML document. Use strict decoding:

- Unknown top-level keys are errors.
- Unknown nested keys are errors.
- Missing optional fields use documented defaults.
- Invalid mode enum values are errors.
- All paths are resolved relative to the config file unless absolute.
- Global config and project config use the same schema, but project config is
  allowed to omit global-only defaults.
- Effective config is produced by merging defaults, global config, project
  config, and CLI flags in that order.

Suggested Go type names:

- `config.File`
- `config.Image`
- `config.Project`
- `config.OpenCode`
- `config.Skills`
- `config.Network`
- `config.Resources`
- `config.Container`

Full conceptual schema:

```yaml
version: 1

state:
  configDir: ~/.config/opencode-sandbox
  dataDir: ~/.local/share/opencode-sandbox
  stateDir: ~/.local/state/opencode-sandbox

image:
  name: ghcr.io/rabbitcybersec/opencode-sandbox:latest
  autoBuild: false
  strictInitImage: ghcr.io/rabbitcybersec/opencode-sandbox-init:latest
  base: debian:bookworm-slim
  installTools: true

project:
  target: /workspace
  readonly: false
  extraMounts: []

opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false
  enabledProviders: []
  disabledProviders: []
  permission:
    edit: ask
    bash: ask

skills:
  importedDir: ~/.config/opencode-sandbox/skills
  projectImportedDir: .opencode-sandbox/skills
  include:
    - "*"
  exclude: []
  exposeProjectSkills: true
  exposeGlobalSkills: false

network:
  inheritGlobal: true
  mode: practical
  blocklist: []
  allowlist: []
  proxyPort: 18080
  dnsPort: 15353
  failClosed: true

resources:
  cpus: 4
  memory: 4g

container:
  namePrefix: opencode-sandbox
  remove: true
  debug: false
```

Do not implement every field in one pass if scope needs trimming, but keep the
schema forward-compatible. For v1, the critical fields are image, project,
OpenCode config mount behavior, skills importedDir, network mode/blocklist, and
resources.

### 8.1 Global and Project Config Precedence

The wrapper must support both global and per-project settings.

Config discovery:

1. Built-in defaults.
2. Global config at `~/.config/opencode-sandbox/config.yaml`.
3. Project config discovered as nearest `.opencode-sandbox.yaml` from the
   project path upward.
4. CLI flags.

Merge rules:

- Scalar values override earlier values only when explicitly set.
- Lists normally replace earlier values, except network lists when
  `network.inheritGlobal: true`.
- With `network.inheritGlobal: true`, project `blocklist` and `allowlist` append
  to global lists.
- With `network.inheritGlobal: false`, project network lists replace global
  lists for that project.
- CLI `--network-mode`, `--readonly`, and resource flags override both configs.

Implementation suggestion:

- Decode YAML into pointer-backed config structs so the loader can distinguish
  "unset" from a zero value.
- Convert merged config into a non-pointer `EffectiveConfig` before command
  generation.

### 8.2 Stateful Host Directories

Use host state deliberately. The wrapper is not stateless.

Global state:

- `~/.config/opencode-sandbox/config.yaml`
- `~/.config/opencode-sandbox/skills`
- `~/.config/opencode-sandbox/policy`
- `~/.local/state/opencode-sandbox/runs`
- `~/.local/share/opencode-sandbox/images` if image metadata is needed

Project state:

- `<project>/.opencode-sandbox.yaml`
- `<project>/.opencode-sandbox/skills`
- `<project>/.opencode-sandbox/skills-manifest.json`

Rules:

- Durable user preferences live in global config.
- Project-specific overrides live with the project.
- Per-run scratch directories live under temp or state and are removed unless
  `--keep` is set.
- Imported skills may be global or project-scoped; project-scoped imports win on
  name conflicts unless the conflict is ambiguous.

## 9. Architecture

Recommended package layout:

```text
cmd/opencode-sandbox/
  main.go

internal/cli/
  root.go
  doctor.go
  init.go
  run.go
  skills.go
  policy.go
  image.go

internal/config/
  config.go
  load.go
  validate.go
  defaults.go

internal/containercmd/
  builder.go
  argv.go
  inspect.go

internal/runtime/
  staging.go
  opencode_config.go
  mounts.go
  names.go

internal/network/
  policy.go
  matcher.go
  practical.go
  strict.go

internal/skills/
  discover.go
  import.go
  manifest.go
  frontmatter.go

internal/doctor/
  checks.go
  report.go

internal/execx/
  runner.go
  fake.go
```

### Module Responsibilities

`internal/config`

- Load YAML.
- Apply defaults.
- Load and merge global/project config.
- Resolve relative paths.
- Validate all input.
- Produce an immutable effective config object.

`internal/containercmd`

- Convert an effective run plan into a `container run` argv slice.
- No filesystem writes.
- No process execution.
- Unit-test this heavily with golden tests.

`internal/runtime`

- Create per-run staging directories.
- Stage OpenCode config/auth.
- Generate temporary OpenCode config overlays.
- Create policy files for practical/strict mode.
- Manage durable wrapper state paths under `.config`/state directories.
- Clean up unless `--keep` is set.

`internal/network`

- Compile blocklist/allowlist matchers.
- Generate practical-mode DNS/proxy config.
- Generate strict-mode init config.
- Explain policy decisions.

`internal/skills`

- Discover skill directories.
- Validate `SKILL.md`.
- Copy selected skills.
- Maintain manifest.

`internal/execx`

- Abstract subprocess execution.
- Let tests inject a fake runner.
- Ensure commands are executed without shell interpolation.

## 10. Filesystem Isolation Design

### 10.1 Mounts

Default mounts:

- Project:
  - host: resolved project path
  - container: `/workspace`
  - readonly: from config or `--readonly`
- Runtime state:
  - host: generated temp/staging directory
  - container: `/sandbox`
  - readonly: false
- Imported skills:
  - host: generated merged skills directory from global and project imports
  - container: `/sandbox/opencode/skills`
  - readonly: true if no writes needed
- Wrapper global state:
  - host: `~/.config/opencode-sandbox`
  - container: `/host/opencode-sandbox-config`
  - readonly: true for normal runs, writable only for explicit config/skills
    commands that intentionally update host state
- Host OpenCode config:
  - host: `~/.config/opencode`
  - container: `/host/opencode-config`
  - readonly: true
- Host OpenCode data:
  - host: likely `~/.local/share/opencode` if needed
  - container: `/host/opencode-data`
  - readonly: true

Do not mount:

- `~`
- `~/.ssh` by default
- shell history
- cloud provider config directories
- package manager caches by default
- Docker socket or other host control sockets

Optional later:

- `--ssh` support, off by default and visible in `--print-command`.
- Extra mounts explicitly listed in config.

### 10.2 Container Root

Use `--read-only` for the container root filesystem.

Add tmpfs mounts:

- `/tmp`
- `/run`
- OpenCode cache/state path if OpenCode requires writable paths
- package manager cache only if necessary

OpenCode should see writable home/config paths under `/sandbox/home` or
`/sandbox/opencode`, not host home.

Runtime should remain stateful from the user's perspective by reading durable
wrapper config/skills/policy from `~/.config/opencode-sandbox`, while each
container run receives a fresh writable sandbox home. This keeps OpenCode from
writing directly to host global config during a run but preserves user choices
across runs.

### 10.3 User and Permissions

Default to a non-root user inside the image if practical.

If Apple `container` mount ownership makes non-root writes painful, implement a
clear compatibility strategy:

1. Prefer matching host UID/GID with `--uid` and `--gid`.
2. If that is unreliable, run as a dedicated container user and document file
   ownership implications.
3. Do not silently run privileged or mount broad host paths to "fix" ownership.

## 11. OpenCode Config and Auth Strategy

User selected: host OpenCode config is allowed.

Security shape:

- Mount host OpenCode config/data read-only.
- Copy only the required files into a per-run sandbox home.
- Never let OpenCode write back directly to host global config.
- Project-local config remains inside the mounted project and follows normal
  OpenCode precedence.
- Wrapper config lives durably at `~/.config/opencode-sandbox/config.yaml`.
- The wrapper's own durable state is separate from OpenCode's host state.

Implementation steps for each run:

1. Create staging dir, for example:
   - macOS temp dir: `/var/folders/.../opencode-sandbox/<run-id>`
   - container path: `/sandbox`
2. Create `/sandbox/home`.
3. Copy selected host OpenCode files from read-only mounts at container startup
   or from the host before launch:
   - `opencode.json`
   - `tui.json`
   - auth/session files if known and needed
4. Generate an overlay `opencode.json` that:
   - sets `autoupdate: false`
   - applies configured `enabled_providers`/`disabled_providers`
   - applies configured permissions
   - points instructions/skills to sandbox paths where needed
5. Set env vars:
   - `HOME=/sandbox/home`
   - `XDG_CONFIG_HOME=/sandbox/home/.config`
   - `XDG_DATA_HOME=/sandbox/home/.local/share`
   - proxy and DNS env vars for practical mode

Important implementation note:

- Before coding this, inspect current OpenCode source/docs to confirm the exact
  auth file locations. The config docs confirm `~/.config/opencode`, but auth
  storage should be verified in the OpenCode repository or by running OpenCode
  in a local test environment.

## 12. Skills Import Design

### 12.1 Principles

- Explicit import, not ambient global skill exposure.
- Project-native skills remain available because they are inside the mounted
  project.
- Imported personal/global skills can be copied into global wrapper state or
  project wrapper state.
- Skill names must be unique in the final exposed skill set.

### 12.2 Discovery

Search these host locations:

- project `.opencode/skills`
- project `.claude/skills`
- project `.agents/skills`
- global `~/.config/opencode/skills`
- global `~/.claude/skills`
- global `~/.agents/skills`

Classify results:

- `active-project`: already in mounted project
- `imported-global`: in `~/.config/opencode-sandbox/skills`
- `imported-project`: in `.opencode-sandbox/skills`
- `available-global`: can be imported, not active by default
- `invalid`: missing `SKILL.md` or invalid frontmatter

### 12.3 Import Rules

- A skill directory is valid if it contains `SKILL.md`.
- Parse frontmatter for `name` and `description`.
- If frontmatter is missing:
  - Warn and use directory name only if OpenCode accepts it.
  - Prefer strict validation if OpenCode docs require frontmatter.
- Copy the whole skill directory.
- Refuse symlinks that point outside the source root.
- Preserve relative assets/scripts/references.
- Compute a manifest hash over copied files.
- Default import destination is global wrapper state:
  `~/.config/opencode-sandbox/skills`.
- `--project <path>` or `--scope project` imports into
  `<project>/.opencode-sandbox/skills`.
- `--scope global` imports into global wrapper state.

### 12.4 Exposing to OpenCode

Simplest approach:

- Generate or mount imported skills at `/workspace/.opencode/skills` only if
  modifying the project is acceptable.

Preferred approach:

- Merge global imported skills and project imported skills into a generated
  per-run skill directory, then mount that into sandbox home at
  `/sandbox/home/.config/opencode/skills`.
- This avoids writing generated skills into the project tree at runtime.
- The `skills import` command stores copied skills either under global wrapper
  state or project `.opencode-sandbox/skills`; the run command merges the
  enabled set.
- If the same skill name appears in both global and project imports, project
  import wins only if the config declares that precedence. Otherwise fail with a
  clear duplicate-name error.

Project-native skills remain in the project mount and OpenCode sees them
normally.

## 13. Network Policy Design

The user wants both practical blocklisting and strict egress control without
making daily development painful. Implement a layered model.

### 13.1 Modes

`network.mode: practical`

- Default.
- Easy to run.
- Blocks normal domain-based traffic.
- Good developer experience.
- Must clearly document bypasses.

`network.mode: strict`

- Opt-in.
- Stronger egress control.
- May require custom init image and deeper runtime integration.
- Should fail closed if setup is incomplete.

`network.mode: off`

- No outbound network.
- Best for local-only work.
- Implement by using a network-less or isolated container setup if Apple
  `container` supports it; otherwise use strict mode with deny-all policy.

### 13.2 Domain Matching

Rules:

- Normalize domains to lowercase.
- Strip trailing dot.
- Convert IDN to ASCII/punycode if implementing full correctness.
- Exact rule `example.com` matches only `example.com`.
- Wildcard rule `*.example.com` matches `api.example.com` and
  `foo.bar.example.com`, but not `example.com`.
- Optional suffix rule `.example.com` can be avoided in v1 to reduce ambiguity.
- URLs passed to `policy test` should be parsed and tested by hostname.

Precedence:

1. Built-in default policy.
2. Global config policy.
3. Project config policy, either appended to or replacing global policy based
   on `network.inheritGlobal`.
4. CLI overrides.
5. Explicit allowlist wins only if configured.
6. Blocklist blocks otherwise.
7. Default allow in practical mode.
8. Default deny in strict mode if `failClosed: true` and policy cannot decide.

Global policy is the normal place for user-wide blocklists. Project policy is
for granular exceptions or additions. Example: globally block analytics and
social domains, then add project-specific blocks for a client repo or disable
inheritance for a repo that needs a fully custom policy.

### 13.3 Practical Mode

Purpose: high-ergonomics blocklist for normal tool traffic.

Suggested implementation:

- Run a tiny DNS/proxy sidecar process inside the same container image or as
  the wrapper entrypoint before OpenCode.
- Configure the container with `--dns 127.0.0.1` only if DNS service runs in a
  way resolvable by the container. Validate this experimentally; if loopback
  DNS from `container run --dns` does not point where expected, use the
  container's resolver config from the entrypoint.
- Set:
  - `HTTP_PROXY=http://127.0.0.1:<proxyPort>`
  - `HTTPS_PROXY=http://127.0.0.1:<proxyPort>`
  - `ALL_PROXY=http://127.0.0.1:<proxyPort>`
  - `NO_PROXY=localhost,127.0.0.1,::1`
- Deny DNS answers for blocklisted domains.
- Deny HTTP CONNECT or HTTP absolute-form proxy requests for blocklisted hosts.
- Log network and command audit attempts to `/sandbox/logs/audit-events.jsonl`.

Practical mode limitations to document:

- Direct IP connections can bypass domain policy.
- Applications that ignore proxy env vars may bypass proxy enforcement.
- Encrypted DNS/DoH can bypass DNS-level blocking if direct egress is available.
- TLS SNI may not always be visible or reliable.

Because of these limitations, practical mode should be described as a developer
guardrail, not a hard security boundary.

### 13.4 Strict Mode

Purpose: strong egress control while still allowing normal OpenCode provider
traffic when permitted.

Suggested implementation path:

- Build a custom init image using `container run --init-image`.
- At VM boot, init configures network rules before starting the OCI process.
- Only allow outbound traffic to the local policy proxy/resolver path.
- Deny direct outbound TCP/UDP from the OpenCode process namespace.
- Force DNS through policy resolver.
- The policy proxy performs:
  - DNS filtering
  - HTTP CONNECT host filtering
  - optional SNI filtering if feasible
  - allow/block decision logging
- Strict mode should fail closed if:
  - init image is missing
  - policy daemon cannot start
  - firewall/eBPF/nftables rules cannot be applied
  - DNS cannot be forced through the resolver

Implementation agents must validate what is actually possible with Apple's
current `container` init-image support. The command reference explicitly says
custom init images may customize boot-time behavior, run VM-level daemons, and
configure eBPF filters, but the exact mechanics need a proof of concept.

Strict-mode MVP:

- Deny all outbound by default.
- Allow outbound only through local proxy.
- Support domain blocklist/allowlist in the proxy.
- Provide an integration test that direct `curl https://blocked.example` fails
  and direct IP egress fails.

### 13.5 Provider Domains

OpenCode providers need network access. Do not hardcode a large fragile allowlist
in v1. Instead:

- Let users configure allowlist/blocklist.
- Provide sample profiles later:
  - OpenAI
  - Anthropic
  - GitHub
  - npm registries
- `policy test` should help users debug blocked providers.

Default blocklist should be empty in generated config, with commented examples.
Avoid shipping a surprising default that breaks first run.

## 14. Container Image Design

The image should be as plain as possible. Start from a minimal base image and
install only the tools needed for OpenCode and the wrapper's policy/runtime
behavior. Avoid a rich "developer workstation" image in v1.

### 14.1 Base Image

Use a slim Linux base that supports arm64 well.

Options:

- Debian slim: best compatibility, larger.
- Alpine: smaller, but may create musl compatibility issues for Node/native
  tooling.

Recommendation: Debian slim for v1. Developer tooling and OpenCode behavior are
more likely to work without strange libc issues.

### 14.2 Installed Tools

Required:

- OpenCode
- Node.js or Bun if required by the chosen OpenCode install path
- Git
- Bash or POSIX shell
- CA certificates
- DNS/proxy policy binary or script

Useful but optional:

- ripgrep
- jq
- curl
- tar
- unzip

Avoid installing a full kitchen sink. The point is isolation with enough tooling
for OpenCode to operate, not a general dev image.

Tooling policy:

- Keep the base image plain.
- Install tools on top in explicit layers.
- Each installed tool must have a reason in the Containerfile comment or image
  docs.
- Do not install language ecosystems beyond what OpenCode needs unless the
  wrapper has an explicit feature requiring them.
- Prefer mounting the project over baking project dependencies into the image.

### 14.3 OpenCode Installation

Current docs list multiple install methods:

- install script
- npm global package `opencode-ai`
- Bun
- pnpm/yarn
- Homebrew

Recommendation for image build:

- Use npm or Bun global install with a pinned version.
- Do not use Homebrew inside Linux image.
- Do not run a remote install script in every user run.
- Disable OpenCode autoupdate in generated config.

The image build command should accept `--opencode-version`.

### 14.4 Entrypoint

Use an entrypoint that:

1. Starts practical-mode policy services if configured.
2. Prepares sandbox home.
3. Copies staged config/auth from read-only mounts if necessary.
4. Execs OpenCode as PID 1 child or through `--init` depending on signal needs.

Keep entrypoint logic small. Larger logic should live in the Go wrapper where it
can be tested on macOS without entering the container.

## 15. Command Construction

Default `container run` shape:

```text
container run
  --rm
  --interactive
  --tty
  --read-only
  --workdir /workspace
  --cpus <cpus>
  --memory <memory>
  --mount type=bind,source=<project>,target=/workspace
  --mount type=bind,source=<staging>,target=/sandbox
  --mount type=bind,source=<host-opencode-config>,target=/host/opencode-config,readonly
  --tmpfs /tmp
  --tmpfs /run
  --env HOME=/sandbox/home
  --env XDG_CONFIG_HOME=/sandbox/home/.config
  --env XDG_DATA_HOME=/sandbox/home/.local/share
  --env OPENCODE_SANDBOX_NETWORK_MODE=<mode>
  <image>
  opencode <args...>
```

Important:

- Construct as `[]string`, never as one shell string.
- Redact secrets in printed commands.
- Print paths exactly enough for debugging.
- Container names should include a stable prefix and short random suffix:
  `opencode-sandbox-<project-slug>-<random>`.

## 16. Security Requirements

### 16.1 Path Safety

- Resolve project path with symlinks where appropriate.
- Reject project paths that do not exist.
- Reject file paths where a directory is required.
- Reject extra mounts outside allowed roots unless explicitly configured.
- Do not follow symlinks during skill import if they escape the source tree.
- Ensure cleanup only deletes wrapper-created temp dirs.

### 16.2 Secrets

- Never log API keys or auth tokens.
- Redact env vars containing:
  - `KEY`
  - `TOKEN`
  - `SECRET`
  - `PASSWORD`
  - `AUTH`
- Host OpenCode config/data mounts are read-only.
- Generated sandbox config should live in temp or project `.opencode-sandbox`
  state, not in global host config.

### 16.3 Policy Transparency

- `--print-command` must show whether strict/practical/off mode is active.
- `policy test` must explain matching rules.
- On blocked request logs, include:
  - timestamp
  - domain/host
  - rule
  - mode
  - action
- Do not log full URLs with query strings by default because they may contain
  secrets.

### 16.4 Failure Modes

- If practical policy service fails to start and `failClosed: true`, do not run
  OpenCode.
- If strict enforcement cannot be established, do not silently fall back to
  practical mode.
- If config staging fails, do not mount host config read-write as fallback.

## 17. Testing Strategy

### 17.1 Unit Tests

Config:

- Defaults apply correctly.
- Global config loads from `~/.config/opencode-sandbox/config.yaml`.
- Project config loads from nearest `.opencode-sandbox.yaml`.
- Global/project config merge semantics are correct.
- Project network policy appends to global policy when `inheritGlobal: true`.
- Project network policy replaces global policy when `inheritGlobal: false`.
- Unknown keys fail.
- Relative paths resolve from config file directory.
- Enum validation for network mode.
- Memory/CPU validation.

Domain matcher:

- Exact match.
- Wildcard match.
- Case normalization.
- Trailing dot normalization.
- URL hostname extraction.
- Allowlist/blocklist precedence.

Container command builder:

- Golden argv for default run.
- Golden argv for readonly project.
- Golden argv for practical mode.
- Golden argv for strict mode.
- Direct OpenCode arg forwarding:
  - `opencode-sandbox --help` maps to `opencode --help`.
  - `opencode-sandbox run "prompt"` maps to `opencode run "prompt"`.
  - `opencode-sandbox . --help` mounts `.` and maps to `opencode --help`.
- `--` OpenCode arg handling remains available as an escape hatch.
- Printed command redacts secrets.

Skills:

- Valid skill detection.
- Missing `SKILL.md` rejected.
- Duplicate skill names rejected.
- Include/exclude globs.
- Symlink escape rejected.
- Manifest hash stable.

Runtime staging:

- Creates expected directory tree.
- Copies only allowed config files.
- Cleans up wrapper temp dirs.
- `--keep` preserves debug state.

### 17.2 Integration Tests With Fake `container`

Create a fake `container` executable in a temp dir and put it first in `PATH`.
The fake should record argv to a file and return configurable exit codes.

Tests:

- `doctor` detects fake `container`.
- `run --dry-run` does not execute fake.
- `run` invokes fake with expected argv.
- root-level direct invocation invokes fake with expected argv.
- Exit code propagates.
- Missing image produces clear error.

### 17.3 Live Integration Tests

Gate behind:

```text
OPENCODE_SANDBOX_LIVE=1
```

Tests:

- `container system start` already running or skipped with message.
- Build image.
- Run `opencode --help` in container.
- Run `opencode-sandbox --help` and verify it reaches containerized OpenCode,
  not only wrapper help.
- Mount temp project and verify container sees files.
- Write a file in temp project and verify host sees it.
- In readonly mode, write attempt fails.
- Practical mode blocks a configured test domain.
- Strict mode blocks direct IP egress if strict support is available.

Do not make live tests part of normal CI until a macOS 26 Apple silicon runner is
available.

### 17.4 Manual Acceptance Checklist

- Fresh clone can run `go test ./...`.
- `opencode-sandbox doctor` gives useful output on a machine without Apple
  `container`.
- `opencode-sandbox init` creates config without overwriting.
- `opencode-sandbox init --global` creates global config under
  `~/.config/opencode-sandbox`.
- `opencode-sandbox image build` builds image.
- `opencode-sandbox run .` starts OpenCode TUI.
- `cd project && opencode-sandbox --help` runs OpenCode help in the sandbox.
- Existing OpenCode auth works without copying the host home directory.
- Imported skill appears in OpenCode.
- Blocklisted domain is blocked in practical mode.
- Project blocklist can add to global blocklist.
- Strict mode either works or fails closed with a clear explanation.

## 18. Implementation Phases

### Phase 1: CLI Skeleton and Config

Deliver:

- Go module.
- CLI command framework.
- `doctor`, `init`, `init --global`, `run --dry-run`, `policy test`,
  `config show`.
- Config load/default/validate/merge.
- Unit tests for config and domain matcher.

Acceptance:

- No real container run required.
- `go test ./...` passes.
- `init` writes expected project YAML.
- `init --global` writes expected global YAML.
- `policy test` explains rules.

### Phase 2: Command Builder and Fake Runtime Tests

Deliver:

- `container run` argv builder.
- Fake runner abstraction.
- `run --print-command`.
- Root-level direct OpenCode arg forwarding.
- Golden tests.
- Exit-code propagation tests.

Acceptance:

- Default run command uses project mount, staging mount, read-only root, tmpfs,
  TTY, interactive, workdir `/workspace`.
- Secrets redacted in printed output.

### Phase 3: Image Build and Basic Run

Deliver:

- Containerfile/Dockerfile for OpenCode image.
- `image build`.
- Basic staging dir.
- Live test gate.

Acceptance:

- On a supported Mac, image builds.
- `opencode-sandbox --help` works from inside a project directory.
- `opencode-sandbox run . --help` works.
- Project mount is visible at `/workspace`.

### Phase 4: OpenCode Config/Auth Staging

Deliver:

- Read-only host config/data mounts.
- Sandbox home generation.
- Generated OpenCode config overlay.
- `autoupdate: false`.
- Permission/provider passthrough.

Acceptance:

- Existing OpenCode auth works in sandbox.
- Host global config is not modified.
- Project `opencode.json` still takes precedence where OpenCode expects it.

### Phase 5: Skills Import

Deliver:

- `skills import`.
- `skills list`.
- Manifest.
- Mount imported skills into sandbox global OpenCode skills path.

Acceptance:

- Imported skill is visible to OpenCode.
- Duplicate names are caught.
- Invalid skills are reported clearly.

### Phase 6: Practical Network Mode

Deliver:

- Domain matcher wired to generated policy.
- Practical DNS/proxy service inside image or entrypoint.
- Proxy env vars.
- Blocked-attempt logging.
- `failClosed` handling.

Acceptance:

- Normal provider access works when not blocklisted.
- Blocklisted domains fail.
- `policy test` matches runtime behavior for normal domain traffic.

### Phase 7: Strict Network Mode Proof of Concept

Deliver:

- Custom init image or equivalent strict enforcement path.
- Deny direct egress.
- Allow egress through policy proxy only.
- Clear failure if unsupported.

Acceptance:

- Direct IP egress fails.
- Blocklisted domain fails.
- Allowed provider domain succeeds.
- If enforcement cannot be established, OpenCode does not start.

### Phase 8: Docs and Polish

Deliver:

- README with quickstart.
- Alias setup docs.
- Security model doc.
- Network mode doc.
- Troubleshooting doc.
- Examples for common providers.

Acceptance:

- New agent/user can set up from README.
- Limitations are clearly documented.
- No misleading claims that practical mode is a hard boundary.

## 19. Suggested Issue Breakdown

Create issues or tasks roughly like this:

1. Scaffold Go module and CLI root.
2. Implement config schema, defaults, validation, and global/project merge.
3. Implement domain matcher and `policy test`.
4. Implement `init` and `init --global`.
5. Implement `config path` and `config show`.
6. Implement root-level OpenCode arg forwarding and project path detection.
7. Implement subprocess runner abstraction.
8. Implement `container run` argv builder.
9. Implement `run --dry-run` and `--print-command`.
10. Add fake `container` integration tests.
11. Add plain-base Containerfile and `image build`.
12. Implement runtime staging directories.
13. Implement host OpenCode config/auth staging.
14. Generate OpenCode config overlay.
15. Implement skill discovery.
16. Implement global/project scoped `skills import`.
17. Implement `skills list`.
18. Mount merged imported skills into sandbox.
19. Implement practical policy files from merged global/project config.
20. Implement practical DNS/proxy service.
21. Wire practical mode into run command.
22. Add blocked-request logs.
23. Prototype strict mode init image.
24. Wire strict mode fail-closed checks.
25. Add live integration test harness.
26. Write user docs, alias docs, and security model.

Keep each issue small enough to finish and test independently.

## 20. Documentation To Add During Implementation

Create these files as features land:

```text
README.md
docs/security-model.md
docs/network-policy.md
docs/opencode-config.md
docs/skills.md
docs/aliases.md
docs/troubleshooting.md
docs/development.md
```

Minimum README sections:

- What this is
- Requirements
- Quickstart
- Shell alias setup for zsh/bash/fish, including
  `alias opencode='opencode-sandbox'`
- Example config
- Global config vs project config
- Network modes
- Skills import
- Security limitations
- Troubleshooting

## 21. Known Risks

### Apple `container` API changes

The command reference used here is from the current main branch. Pin behavior to
a released Apple `container` version before shipping and update docs/tests
accordingly.

### OpenCode auth path uncertainty

OpenCode docs clearly document config paths, but auth/session storage should be
verified before implementation. Build a small discovery test or inspect the
OpenCode source.

### Practical mode overclaiming

Practical mode is useful but bypassable. Do not market it as strict isolation.
Use strict mode for stronger claims.

### Strict mode complexity

Strict egress depends on custom init-image mechanics and guest networking. Treat
Phase 7 as a proof of concept before promising it in the README.

### File ownership

Writes from a Linux container to a macOS bind mount may have ownership quirks.
Test early with real Apple `container` and decide whether UID/GID mapping is
enough.

### OpenCode plugin/MCP behavior

Plugins and MCP servers can open new capabilities and network paths. Keep them
explicitly configured and visible in generated config.

## 22. Defaults To Preserve

These are product decisions from the planning conversation:

- CLI implementation in Go.
- Mount project folder at `/workspace`.
- If no project folder is supplied, mount the current working directory.
- Direct invocation forwards to OpenCode; users should not need
  `opencode-sandbox -- opencode`.
- Wrapper state is durable under `~/.config/opencode-sandbox`.
- The image starts from a plain slim base and installs only required tools on
  top.
- Global config supplies user-wide blocklists and defaults.
- Project config can add to or replace global blocklists.
- Host OpenCode config/data allowed, but only read-only/staged.
- Skill import is explicit.
- Practical network mode is default.
- Strict network mode is opt-in and fail-closed.
- Do not implement a GUI for v1.

## 23. Definition of Done for v1

v1 is complete when:

- A user on supported macOS can build/install the wrapper.
- `doctor` explains readiness.
- `init` creates project config.
- `init --global` creates global config under
  `~/.config/opencode-sandbox/config.yaml`.
- `image build` builds the OpenCode image.
- `run .` starts OpenCode in `/workspace`.
- `opencode-sandbox --help` from a project directory runs OpenCode help in
  the sandbox.
- Host OpenCode auth/config works without mounting the full home directory.
- Imported skills are available to OpenCode.
- Practical blocklist blocks ordinary domain traffic.
- Global and project blocklists merge according to documented precedence.
- Strict mode has either a working implementation or is clearly marked
  experimental behind an explicit flag.
- Tests cover config, command generation, policy matching, skills import, and
  fake runtime behavior.
- Docs explain setup, security model, network modes, and limitations.
