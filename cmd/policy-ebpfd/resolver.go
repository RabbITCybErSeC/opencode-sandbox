package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// MapManager abstracts eBPF map updates so the resolver can be tested
// without a real Linux eBPF runtime.
type MapManager interface {
	UpdateBlocked(ip net.IP, rule string) error
	UpdateAllowed(ip net.IP, rule string) error
	RemoveBlocked(ip net.IP) error
	RemoveAllowed(ip net.IP) error
}

// ResolverEntry tracks a resolved domain and its metadata.
type ResolverEntry struct {
	Domain    string
	IPs       []net.IP
	Rule      string    // the matching rule (e.g. "*.segment.io")
	Blocked   bool      // true if this came from the blocklist
	ExpiresAt time.Time
}

// Resolver converts domain rules into IP map updates.
type Resolver struct {
	mu       sync.RWMutex
	entries  map[string]*ResolverEntry // key: domain
	blocklist []string
	allowlist []string
	ttlMin   time.Duration
	ttlMax   time.Duration
	manager  MapManager
	resolver *net.Resolver
}

// NewResolver builds a resolver from policy bundle rules.
func NewResolver(blocklist, allowlist []string, ttlMin, ttlMax int, manager MapManager) *Resolver {
	return &Resolver{
		entries:   make(map[string]*ResolverEntry),
		blocklist: normalizeRules(blocklist),
		allowlist: normalizeRules(allowlist),
		ttlMin:    time.Duration(ttlMin) * time.Second,
		ttlMax:    time.Duration(ttlMax) * time.Second,
		manager:   manager,
		resolver:  net.DefaultResolver,
	}
}

// Run performs the initial resolution of exact domains and starts a
// background refresh loop. It blocks until the context is cancelled.
func (r *Resolver) Run(ctx context.Context) error {
	// Resolve exact domains immediately.
	for _, rule := range r.blocklist {
		if !isWildcard(rule) {
			if err := r.resolveAndUpdate(ctx, rule, rule, true); err != nil {
				fmt.Fprintf(os.Stderr, "resolver: failed to resolve %q: %v\n", rule, err)
			}
		}
	}
	for _, rule := range r.allowlist {
		if !isWildcard(rule) {
			if err := r.resolveAndUpdate(ctx, rule, rule, false); err != nil {
				fmt.Fprintf(os.Stderr, "resolver: failed to resolve %q: %v\n", rule, err)
			}
		}
	}

	// Background refresh loop.
	ticker := time.NewTicker(r.ttlMin)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.refreshStale(ctx)
		}
	}
}

// resolveAndUpdate resolves a single domain and updates maps.
func (r *Resolver) resolveAndUpdate(ctx context.Context, domain, rule string, blocked bool) error {
	addrs, err := r.resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return err
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old entries for this domain.
	if old, ok := r.entries[domain]; ok {
		for _, ip := range old.IPs {
			if old.Blocked {
				r.manager.RemoveBlocked(ip)
			} else {
				r.manager.RemoveAllowed(ip)
			}
		}
	}

	ttl := r.ttlMin
	if len(addrs) > 0 && addrs[0].IP != nil {
		// In a full implementation we would parse DNS TTL.
		// For the POC we use the configured minimum.
		_ = ttl
	}

	entry := &ResolverEntry{
		Domain:    domain,
		IPs:       ips,
		Rule:      rule,
		Blocked:   blocked,
		ExpiresAt: time.Now().Add(r.ttlMin),
	}
	r.entries[domain] = entry

	for _, ip := range ips {
		if blocked {
			if err := r.manager.UpdateBlocked(ip, rule); err != nil {
				fmt.Fprintf(os.Stderr, "resolver: failed to update blocked map for %s: %v\n", ip, err)
			}
		} else {
			if err := r.manager.UpdateAllowed(ip, rule); err != nil {
				fmt.Fprintf(os.Stderr, "resolver: failed to update allowed map for %s: %v\n", ip, err)
			}
		}
	}

	return nil
}

// refreshStale re-resolves entries that have passed their TTL.
func (r *Resolver) refreshStale(ctx context.Context) {
	r.mu.Lock()
	now := time.Now()
	var stale []string
	for domain, entry := range r.entries {
		if now.After(entry.ExpiresAt) {
			stale = append(stale, domain)
		}
	}
	r.mu.Unlock()

	for _, domain := range stale {
		r.mu.RLock()
		entry := r.entries[domain]
		r.mu.RUnlock()
		if entry == nil {
			continue
		}
		if err := r.resolveAndUpdate(ctx, domain, entry.Rule, entry.Blocked); err != nil {
			fmt.Fprintf(os.Stderr, "resolver: refresh failed for %q: %v\n", domain, err)
		}
	}
}

// LookupRule checks if a hostname matches any blocklist or allowlist rule.
// It returns the matched rule, a boolean indicating if it was blocked, and
// a boolean indicating if a match was found.
func (r *Resolver) LookupRule(hostname string) (rule string, blocked, matched bool) {
	host := strings.ToLower(strings.TrimSuffix(hostname, "."))

	// Allowlist wins.
	for _, rule := range r.allowlist {
		if matchDomain(host, rule) {
			return rule, false, true
		}
	}
	for _, rule := range r.blocklist {
		if matchDomain(host, rule) {
			return rule, true, true
		}
	}
	return "", false, false
}

func normalizeRules(rules []string) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = strings.ToLower(strings.TrimSpace(r))
	}
	return out
}

func isWildcard(rule string) bool {
	return strings.HasPrefix(rule, "*.")
}

func matchDomain(domain, rule string) bool {
	if strings.HasPrefix(rule, "*.") {
		suffix := rule[2:]
		return strings.HasSuffix(domain, "."+suffix)
	}
	return domain == rule
}
