package main

import "testing"

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
