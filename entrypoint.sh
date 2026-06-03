#!/bin/bash
set -e

# Entrypoint for the opencode-sandbox container.
# Starts the policy proxy (if in practical/strict mode), supervises OpenCode,
# and stops the proxy before exiting.

NETWORK_MODE="${OPENCODE_SANDBOX_NETWORK_MODE:-practical}"
NETWORK_BACKEND="${OPENCODE_SANDBOX_NETWORK_BACKEND:-proxy}"
POLICY_FILE="${OPENCODE_SANDBOX_POLICY_FILE:-/sandbox/policy.json}"
POLICY_LOG="${POLICY_LOG_FILE:-/sandbox/logs/network.log}"
SANDBOX_HOME="${OPENCODE_SANDBOX_HOME:-/sandbox/home}"
PROXY_PID=""
OPENCODE_PID=""

cleanup_proxy() {
    if [ -n "$PROXY_PID" ] && kill -0 "$PROXY_PID" 2>/dev/null; then
        kill -TERM "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi
}

forward_signal() {
    if [ -n "$OPENCODE_PID" ] && kill -0 "$OPENCODE_PID" 2>/dev/null; then
        kill -TERM "$OPENCODE_PID" 2>/dev/null || true
    fi
}

trap forward_signal INT TERM

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
mkdir -p "$SANDBOX_HOME/.config/opencode"

# Run OpenCode with all forwarded arguments, then clean up the proxy.
opencode "$@" &
OPENCODE_PID=$!

set +e
wait "$OPENCODE_PID"
STATUS=$?
set -e

cleanup_proxy
exit "$STATUS"
