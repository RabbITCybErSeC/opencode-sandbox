package main

import (
	"net"
	"testing"
)

func TestIPv4KeyUsesNetworkByteOrder(t *testing.T) {
	key, ok := ipv4Key(net.ParseIP("192.0.2.1"))
	if !ok {
		t.Fatal("expected IPv4 key")
	}
	if key != 0xc0000201 {
		t.Fatalf("unexpected key %#x", key)
	}
}

func TestIPv4KeyIgnoresIPv6(t *testing.T) {
	if _, ok := ipv4Key(net.ParseIP("2001:db8::1")); ok {
		t.Fatal("expected IPv6 to be ignored by IPv4 map manager")
	}
}

func TestCgroupConnect4InstructionsReferenceMaps(t *testing.T) {
	insns := cgroupConnect4Instructions()
	refs := map[string]bool{}
	symbols := map[string]bool{}
	for _, ins := range insns {
		if ref := ins.Reference(); ref != "" {
			refs[ref] = true
		}
		if sym := ins.Symbol(); sym != "" {
			symbols[sym] = true
		}
	}
	for _, want := range []string{"allowed_ipv4", "blocked_ipv4", "settings"} {
		if !refs[want] {
			t.Fatalf("expected map reference %q in instructions", want)
		}
	}
	for _, want := range []string{"allow", "deny"} {
		if !symbols[want] {
			t.Fatalf("expected symbol %q in instructions", want)
		}
	}
	if _, err := insns.SymbolOffsets(); err != nil {
		t.Fatalf("expected instruction symbols to resolve: %v", err)
	}
}
