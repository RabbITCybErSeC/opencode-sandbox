# Sandboxed OpenCode container image.
# Debian slim base with OpenCode plus fetched Rust, Go, and JavaScript/TypeScript toolchains.

ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-bookworm AS builder

# Build the policy proxy with the current Go 1.26 patch release.
WORKDIR /build
COPY go.mod go.sum ./
COPY cmd/policy-proxy ./cmd/policy-proxy
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o policy-proxy ./cmd/policy-proxy

# ---

FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/RabbITCybErSeC/opencode-sandbox"

# Install small runtime dependencies and make.
# - git: required for OpenCode repository operations
# - ca-certificates: required for HTTPS outbound connections
# - curl, wget: downloads upstream language distributions
# - bash: required for entrypoint and general shell use
# - procps: provides ps for debugging
# - ripgrep: fast text search, commonly used by OpenCode
# - jq: JSON parsing for upstream release metadata
# - make: commonly used project task runner without pulling in a full C/C++ build stack
# - libatomic1: required by upstream Node.js binaries on slim Debian images
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    libatomic1 \
    make \
    procps \
    ripgrep \
    tar \
    wget \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Reuse the prebuilt Go toolchain from the builder image instead of downloading or compiling it again.
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Install the latest current upstream Node.js release.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
        amd64) node_arch="x64" ;; \
        arm64) node_arch="arm64" ;; \
        *) echo "unsupported architecture for Node.js: $arch" >&2; exit 1 ;; \
    esac; \
    node_version="$(curl -fsSL 'https://nodejs.org/dist/index.json' | jq -r '.[0].version')"; \
    curl -fsSL -o /tmp/node.tar.xz "https://nodejs.org/dist/${node_version}/node-${node_version}-linux-${node_arch}.tar.xz"; \
    tar -C /usr/local --strip-components=1 -xJf /tmp/node.tar.xz; \
    rm -f /tmp/node.tar.xz; \
    npm install -g typescript ts-node; \
    npm cache clean --force

# Install the latest upstream stable Rust toolchain.
ENV RUSTUP_HOME="/usr/local/rustup"
ENV CARGO_HOME="/usr/local/cargo"
ENV PATH="${CARGO_HOME}/bin:${PATH}"

RUN set -eux; \
    curl -fsSL -o /tmp/rustup-init.sh https://sh.rustup.rs; \
    sh /tmp/rustup-init.sh -y --profile minimal --default-toolchain stable; \
    rm -f /tmp/rustup-init.sh; \
    rustup component add rustfmt clippy; \
    rm -rf "$CARGO_HOME"/registry "$CARGO_HOME"/git "$RUSTUP_HOME"/downloads "$RUSTUP_HOME"/tmp

# Expose toolchain binaries through /usr/local/bin so they are available even
# when a runtime overrides the image entrypoint or PATH handling.
RUN set -eux; \
    ln -sf /usr/local/go/bin/go /usr/local/bin/go; \
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt; \
    rust_toolchain_bin="$(find "$RUSTUP_HOME/toolchains" -mindepth 2 -maxdepth 2 -type d -name bin | head -n 1)"; \
    test -n "$rust_toolchain_bin"; \
    for bin in "$rust_toolchain_bin"/*; do \
        ln -sf "$bin" "/usr/local/bin/$(basename "$bin")"; \
    done

# Install OpenCode globally via npm.
# Version can be pinned at build time with --build-arg OPENCODE_VERSION=x.y.z
ARG OPENCODE_VERSION=latest
RUN if [ "$OPENCODE_VERSION" = "latest" ]; then \
        npm install -g opencode-ai; \
    else \
        npm install -g opencode-ai@"$OPENCODE_VERSION"; \
    fi

# Print resolved tool versions so builds show exactly what "latest" selected.
RUN set -eux; \
    rustc --version; \
    cargo --version; \
    rustfmt --version; \
    clippy-driver --version; \
    go version; \
    node --version; \
    npm --version; \
    tsc --version; \
    ts-node --version; \
    make --version | head -n 1; \
    command -v opencode

# Copy policy proxy from builder.
COPY --from=builder /build/policy-proxy /usr/local/bin/policy-proxy

# Create a non-root user for running OpenCode.
RUN useradd -m -s /bin/bash opencode

# Set up writable directories.
RUN mkdir -p /sandbox/home /sandbox/opencode /sandbox/logs \
    && chown -R opencode:opencode /sandbox /usr/local/cargo /usr/local/rustup

# Working directory inside the container.
WORKDIR /workspace

# Copy entrypoint script.
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Default to running as the opencode user.
USER opencode

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]