package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
)

func TestProxyDecisionHonorsDefaultAction(t *testing.T) {
	p := &Proxy{Policy: Policy{Mode: "practical", DefaultAction: "deny"}}
	decision, rule := p.decide("unlisted.example.com")
	if decision != "block" || rule != "default-deny" {
		t.Fatalf("expected default deny block, got decision=%s rule=%s", decision, rule)
	}
}

func TestProxyDecisionAllowlistWins(t *testing.T) {
	p := &Proxy{Policy: Policy{
		Mode:          "practical",
		DefaultAction: "deny",
		Blocklist:     []string{"*.example.com"},
		Allowlist:     []string{"api.example.com"},
	}}
	decision, rule := p.decide("api.example.com")
	if decision != "allow" || rule != "api.example.com" {
		t.Fatalf("expected allowlist to win, got decision=%s rule=%s", decision, rule)
	}
}

func TestProxyDecisionBlocksRule(t *testing.T) {
	p := &Proxy{Policy: Policy{
		Mode:          "practical",
		DefaultAction: "allow",
		Blocklist:     []string{"*.bad.example.com"},
	}}
	decision, rule := p.decide("cdn.bad.example.com")
	if decision != "block" || rule != "*.bad.example.com" {
		t.Fatalf("expected blocklist match, got decision=%s rule=%s", decision, rule)
	}
}

func TestProxyLogsBlockedHTTPAsJSON(t *testing.T) {
	proxy, path := testProxyWithLog(t, Policy{
		Mode:          "practical",
		DefaultAction: "allow",
		Blocklist:     []string{"blocked.example.com"},
	})

	req := httptest.NewRequest(http.MethodGet, "http://blocked.example.com/path?token=secret", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
	event := readOneAuditEvent(t, path)
	if event.EventType != auditlog.EventNetworkConnect || event.Decision != "block" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.Host != "blocked.example.com" || event.Method != "HTTP" {
		t.Fatalf("unexpected host/method: %+v", event)
	}
	data, _ := os.ReadFile(path)
	for _, forbidden := range []string{"/path", "token", "secret"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("audit event leaked %q: %s", forbidden, data)
		}
	}
}

func TestProxyLogsAllowedHTTPAsJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	proxy, path := testProxyWithLog(t, Policy{Mode: "practical", DefaultAction: "allow"})
	req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected upstream status, got %d", rec.Code)
	}
	event := readOneAuditEvent(t, path)
	if event.Decision != "allow" || event.Reason != "policy-allow" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestProxyLogsUpstreamErrorAsJSON(t *testing.T) {
	proxy, path := testProxyWithLog(t, Policy{Mode: "practical", DefaultAction: "allow"})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1/", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d", rec.Code)
	}
	event := readOneAuditEvent(t, path)
	if event.Decision != "error" || event.Reason != "upstream-error" || event.Error == "" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func testProxyWithLog(t *testing.T, policy Policy) (*Proxy, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, auditlog.DefaultFileName)
	writer, err := auditlog.NewWriter(path, "", false, auditlog.RotationConfig{MaxBytes: auditlog.DefaultRotationMaxBytes, MaxFiles: auditlog.DefaultRotationMaxFiles})
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	policy.RunID = "run-1"
	policy.Project.Name = "proj"
	policy.Network.Backend = "proxy"
	return &Proxy{Policy: policy, Logger: writer}, path
}

func readOneAuditEvent(t *testing.T, path string) auditlog.Event {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one event, got %d: %s", len(lines), data)
	}
	var event auditlog.Event
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	return event
}
