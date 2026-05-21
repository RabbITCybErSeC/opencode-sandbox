#!/bin/bash
set -e

# Entrypoint for the opencode-sandbox container.
# Starts the policy proxy (if in practical/strict mode) then execs OpenCode.

NETWORK_MODE="${OPENCODE_SANDBOX_NETWORK_MODE:-practical}"
NETWORK_BACKEND="${OPENCODE_SANDBOX_NETWORK_BACKEND:-proxy}"
POLICY_FILE="${OPENCODE_SANDBOX_POLICY_FILE:-/sandbox/policy.json}"
POLICY_LOG="${POLICY_LOG_FILE:-/sandbox/logs/network.log}"

if [ "$NETWORK_BACKEND" = "proxy" ] && { [ "$NETWORK_MODE" = "practical" ] || [ "$NETWORK_MODE" = "strict" ]; }; then
    if [ -f "$POLICY_FILE" ]; then
        echo "Starting policy proxy..."
        policy-proxy "$POLICY_FILE" &
        PROXY_PID=$!

        # Wait briefly for proxy to start.
        sleep 0.5

        # Verify proxy is running.
        if ! kill -0 $PROXY_PID 2>/dev/null; then
            echo "Policy proxy failed to start."
            if [ "${OPENCODE_SANDBOX_FAIL_CLOSED:-true}" = "true" ]; then
                exit 1
            fi
        fi
    else
        echo "Warning: policy file not found at $POLICY_FILE"
        if [ "${OPENCODE_SANDBOX_FAIL_CLOSED:-true}" = "true" ]; then
            exit 1
        fi
    fi
fi

# Ensure sandbox home exists.
mkdir -p /sandbox/home/.config/opencode

# Exec OpenCode with all forwarded arguments.
exec opencode "$@"
