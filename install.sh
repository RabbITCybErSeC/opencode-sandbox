#!/usr/bin/env sh
set -eu

repo_url="${OPENCODE_SANDBOX_REPO:-https://github.com/RabbITCybErSeC/opencode-sandbox.git}"
src_dir="${OPENCODE_SANDBOX_DIR:-"$HOME/.local/share/opencode-sandbox-src"}"
bin_path="${OPENCODE_SANDBOX_BIN:-"$HOME/.local/bin/opencode-sandbox"}"
runtime_image="${OPENCODE_SANDBOX_IMAGE:-ghcr.io/rabbitcybersec/opencode-sandbox:latest}"
strict_init_image="${OPENCODE_SANDBOX_INIT_IMAGE:-ghcr.io/rabbitcybersec/opencode-sandbox-init:latest}"
pull_image="${OPENCODE_SANDBOX_PULL_IMAGE:-1}"
pull_strict_init="${OPENCODE_SANDBOX_PULL_STRICT_INIT:-}"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		printf 'Error: required command not found: %s\n' "$1" >&2
		return 1
	fi
}

check_container_available() {
	if [ "$(uname -s)" != "Darwin" ]; then
		printf 'Error: Apple container requires macOS.\n' >&2
		return 1
	fi

	if ! command -v container >/dev/null 2>&1; then
		printf 'Error: Apple container binary not found in PATH. Install Apple container and ensure it is available on PATH.\n' >&2
		return 1
	fi

	if ! container_version_output="$(container system version 2>&1)"; then
		printf 'Error: Apple container is not available on this macOS system.\n' >&2
		printf 'The container tool may be uninstalled, unsupported on this macOS version, or not initialized correctly.\n' >&2
		if [ -n "$container_version_output" ]; then
			printf '%s\n' "$container_version_output" >&2
		fi
		return 1
	fi
}

is_disabled() {
	case "$1" in
		0|false|False|FALSE|no|No|NO) return 0 ;;
		*) return 1 ;;
	esac
}

is_enabled() {
	case "$1" in
		1|true|True|TRUE|yes|Yes|YES) return 0 ;;
		*) return 1 ;;
	esac
}

pull_image_ref() {
	printf 'Pulling %s\n' "$1"
	"$bin_path" image pull --tag "$1"
}

need_cmd git
need_cmd go
check_container_available

if [ -d "$src_dir/.git" ]; then
	printf 'Updating opencode-sandbox in %s\n' "$src_dir"
	git -C "$src_dir" pull --ff-only
elif [ -e "$src_dir" ]; then
	printf 'Error: %s exists but is not a git checkout.\n' "$src_dir" >&2
	printf 'Set OPENCODE_SANDBOX_DIR to another path or move the existing path aside.\n' >&2
	exit 1
else
	printf 'Cloning opencode-sandbox into %s\n' "$src_dir"
	mkdir -p "$(dirname "$src_dir")"
	git clone "$repo_url" "$src_dir"
fi

printf 'Building opencode-sandbox to %s\n' "$bin_path"
mkdir -p "$(dirname "$bin_path")"
ldflags="-X github.com/RabbITCybErSeC/opencode-sandbox/internal/cli.installedSourceDir=$src_dir"
(cd "$src_dir" && go build -ldflags "$ldflags" -o "$bin_path" ./cmd/opencode-sandbox)

if is_disabled "$pull_image"; then
	printf 'Skipping runtime image pull because OPENCODE_SANDBOX_PULL_IMAGE=%s\n' "$pull_image"
else
	pull_image_ref "$runtime_image"
fi

if is_enabled "$pull_strict_init"; then
	pull_image_ref "$strict_init_image"
elif is_disabled "$pull_strict_init"; then
	printf 'Skipping strict init image pull because OPENCODE_SANDBOX_PULL_STRICT_INIT=%s\n' "$pull_strict_init"
elif [ -r /dev/tty ] && [ -w /dev/tty ]; then
	printf 'Pull strict eBPF init image now? [y/N] ' >/dev/tty
	read answer </dev/tty || answer=
	case "$answer" in
		y|Y|yes|YES|Yes) pull_image_ref "$strict_init_image" ;;
		*) printf 'Skipping strict init image pull. You can fetch it later with: %s image pull --strict-init\n' "$bin_path" ;;
	esac
fi

alias_path="$bin_path"
case "$alias_path" in
	"$HOME"/*) alias_path="\$HOME/${alias_path#"$HOME/"}" ;;
esac

cat <<EOF

Installed opencode-sandbox.

Add this alias to your shell profile:

  alias sopencode="$alias_path"

Then open a new shell or run the alias command above, and try:

  sopencode doctor
  sopencode image pull
  sopencode run .

For local source builds or custom OpenCode versions:

  sopencode image build

EOF
