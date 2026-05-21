package main

import (
	"context"
	"net"
	"testing"
	"time"
)

// fakeMapManager is a test double for MapManager.
type fakeMapManager struct {
	blocked map[string]string // ip -> rule
	allowed map[string]string // ip -> rule
}

func newFakeMapManager() *fakeMapManager {
	return &fakeMapManager{
		blocked: make(map[string]string),
		allowed: make(map[string]string),
	}
}

func (f *fakeMapManager) UpdateBlocked(ip net.IP, rule string) error {
	f.blocked[ip.String()] = rule
	return nil
}

func (f *fakeMapManager) UpdateAllowed(ip net.IP, rule string) error {
	f.allowed[ip.String()] = rule
	return nil
}

func (f *fakeMapManager) RemoveBlocked(ip net.IP) error {
	delete(f.blocked, ip.String())
	return nil
}

func (f *fakeMapManager) RemoveAllowed(ip net.IP) error {
	delete(f.allowed, ip.String())
	return nil
}

func TestResolverLookupRuleExact(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"bad.example.com"},
		[]string{"good.example.com"},
		30, 300,
		mgr,
	)

	rule, blocked, matched := r.LookupRule("bad.example.com")
	if !matched || !blocked || rule != "bad.example.com" {
		t.Errorf("expected blocked exact match, got rule=%q blocked=%v matched=%v", rule, blocked, matched)
	}

	rule, blocked, matched = r.LookupRule("good.example.com")
	if !matched || blocked || rule != "good.example.com" {
		t.Errorf("expected allowed exact match, got rule=%q blocked=%v matched=%v", rule, blocked, matched)
	}
}

func TestResolverLookupRuleWildcard(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"*.bad.example.com"},
		[]string{},
		30, 300,
		mgr,
	)

	rule, blocked, matched := r.LookupRule("api.bad.example.com")
	if !matched || !blocked || rule != "*.bad.example.com" {
		t.Errorf("expected wildcard block match, got rule=%q blocked=%v matched=%v", rule, blocked, matched)
	}

	_, _, matched = r.LookupRule("bad.example.com")
	if matched {
		t.Error("wildcard should not match the base domain")
	}
}

func TestResolverAllowlistWins(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"example.com"},
		[]string{"example.com"},
		30, 300,
		mgr,
	)

	rule, blocked, matched := r.LookupRule("example.com")
	if !matched || blocked {
		t.Errorf("allowlist should win, got rule=%q blocked=%v", rule, blocked)
	}
}

func TestResolverResolveAndUpdate(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"localhost"},
		[]string{},
		30, 300,
		mgr,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.resolveAndUpdate(ctx, "localhost", "localhost", true); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	// localhost should resolve to at least 127.0.0.1
	if _, ok := mgr.blocked["127.0.0.1"]; !ok {
		t.Logf("blocked map: %v", mgr.blocked)
		// On some systems localhost may resolve to ::1 first; that's okay for the test.
		if _, ok := mgr.blocked["::1"]; !ok {
			t.Error("expected localhost to be resolved and added to blocked map")
		}
	}
}

func TestResolverRefreshReplacesOldIPs(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"localhost"},
		[]string{},
		1, 300,
		mgr,
	)

	ctx := context.Background()

	// First resolution.
	if err := r.resolveAndUpdate(ctx, "localhost", "localhost", true); err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}

	// Force expiration.
	r.mu.Lock()
	r.entries["localhost"].ExpiresAt = time.Now().Add(-time.Second)
	r.mu.Unlock()

	// Refresh should replace.
	r.refreshStale(ctx)

	// The entry should still exist and be refreshed.
	r.mu.RLock()
	entry := r.entries["localhost"]
	r.mu.RUnlock()

	if entry == nil {
		t.Fatal("expected entry to exist after refresh")
	}
	if time.Now().After(entry.ExpiresAt) {
		t.Error("expected entry to be refreshed with future expiry")
	}
}

func TestResolverRemoveOnRefresh(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{"localhost"},
		[]string{},
		1, 300,
		mgr,
	)

	ctx := context.Background()

	// Manually inject an entry with a stale IP.
	r.mu.Lock()
	r.entries["localhost"] = &ResolverEntry{
		Domain:    "localhost",
		IPs:       []net.IP{net.ParseIP("192.0.2.1")},
		Rule:      "localhost",
		Blocked:   true,
		ExpiresAt: time.Now().Add(-time.Second),
	}
	mgr.blocked["192.0.2.1"] = "localhost"
	r.mu.Unlock()

	// Refresh should remove the stale IP and add the real one.
	r.refreshStale(ctx)

	if _, ok := mgr.blocked["192.0.2.1"]; ok {
		t.Error("expected stale IP to be removed from blocked map")
	}
}

func TestResolverRunContextCancel(t *testing.T) {
	mgr := newFakeMapManager()
	r := NewResolver(
		[]string{},
		[]string{},
		30, 300,
		mgr,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := r.Run(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
