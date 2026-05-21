#!/usr/bin/env sh
set -eu

repo="${OPENCODE_SANDBOX_REPO:-RabbITCybErSeC/opencode-sandbox}"
case "$repo" in
	https://github.com/*)
		repo="${repo#https://github.com/}"
		repo="${repo%.git}"
		;;
	git@github.com:*)
		repo="${repo#git@github.com:}"
		repo="${repo%.git}"
		;;
esac
version="${OPENCODE_SANDBOX_VERSION:-}"
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

detect_asset() {
	os_name="$(uname -s)"
	machine="$(uname -m)"

	if [ "$os_name" != "Darwin" ]; then
		printf 'Error: Apple container requires macOS.\n' >&2
		return 1
	fi

	case "$machine" in
		arm64|aarch64)
			arch="arm64"
			;;
		x86_64|amd64)
			arch="amd64"
			;;
		*)
			printf 'Error: unsupported macOS architecture: %s\n' "$machine" >&2
			return 1
			;;
	esac

	printf 'opencode-sandbox_darwin_%s.tar.gz\n' "$arch"
}

release_url() {
	asset="$1"
	if [ -n "$version" ]; then
		printf 'https://github.com/%s/releases/download/%s/%s\n' "$repo" "$version" "$asset"
	else
		printf 'https://github.com/%s/releases/latest/download/%s\n' "$repo" "$asset"
	fi
}

checksum_url() {
	if [ -n "$version" ]; then
		printf 'https://github.com/%s/releases/download/%s/checksums.txt\n' "$repo" "$version"
	else
		printf 'https://github.com/%s/releases/latest/download/checksums.txt\n' "$repo"
	fi
}

check_container_available() {
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

sha256_file() {
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	elif command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		return 1
	fi
}

verify_checksum() {
	archive="$1"
	asset="$2"
	checksums="$3"

	expected="$(awk -v file="$asset" '$2 == file { print $1 }' "$checksums" | head -n 1)"
	if [ -z "$expected" ]; then
		printf 'Warning: no checksum entry for %s; skipping checksum verification.\n' "$asset" >&2
		return 0
	fi

	if ! actual="$(sha256_file "$archive")"; then
		printf 'Warning: shasum or sha256sum not found; skipping checksum verification.\n' >&2
		return 0
	fi

	if [ "$actual" != "$expected" ]; then
		printf 'Error: checksum mismatch for %s\n' "$asset" >&2
		printf 'Expected: %s\n' "$expected" >&2
		printf 'Actual:   %s\n' "$actual" >&2
		return 1
	fi

	printf 'Verified checksum for %s\n' "$asset"
}

download_binary() {
	asset="$1"
	tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/opencode-sandbox-install.XXXXXX")"
	trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

	archive="$tmpdir/$asset"
	extract_dir="$tmpdir/extract"
	checksums="$tmpdir/checksums.txt"
	mkdir -p "$extract_dir"

	url="$(release_url "$asset")"
	printf 'Downloading %s\n' "$url"
	curl -fL "$url" -o "$archive"

	if curl -fL "$(checksum_url)" -o "$checksums"; then
		verify_checksum "$archive" "$asset" "$checksums"
	else
		printf 'Warning: checksums.txt not available; skipping checksum verification.\n' >&2
	fi

	tar -xzf "$archive" -C "$extract_dir"
	if [ ! -f "$extract_dir/opencode-sandbox" ]; then
		printf 'Error: release archive did not contain opencode-sandbox binary.\n' >&2
		return 1
	fi

	printf 'Installing opencode-sandbox to %s\n' "$bin_path"
	mkdir -p "$(dirname "$bin_path")"
	cp -f "$extract_dir/opencode-sandbox" "$bin_path"
	chmod 0755 "$bin_path"
}

pull_image_ref() {
	printf 'Pulling %s\n' "$1"
	"$bin_path" image pull --tag "$1"
}

need_cmd curl
need_cmd tar

asset="$(detect_asset)"
check_container_available
download_binary "$asset"

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

  git clone https://github.com/$repo.git
  sopencode image build --context ./opencode-sandbox

EOF
