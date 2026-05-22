package main

import (
	"encoding/binary"
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
	for _, want := range []string{"allowed_ipv4", "blocked_ipv4", "settings", "network_events"} {
		if !refs[want] {
			t.Fatalf("expected map reference %q in instructions", want)
		}
	}
	for _, want := range []string{"allow", "deny", "check_blocked", "check_default", "default_deny"} {
		if !symbols[want] {
			t.Fatalf("expected symbol %q in instructions", want)
		}
	}
	if _, err := insns.SymbolOffsets(); err != nil {
		t.Fatalf("expected instruction symbols to resolve: %v", err)
	}
}

func TestNetworkEventFromSampleResolvesDecisionAndRule(t *testing.T) {
	bundle := &PolicyBundle{RunID: "run-1"}
	bundle.Project.Name = "proj"
	handle := &enforcementHandle{rules: map[uint32]string{7: "*.example.com"}}
	raw := make([]byte, 28)
	binary.LittleEndian.PutUint32(raw[0:4], 42)
	binary.LittleEndian.PutUint32(raw[4:8], 0xcb00710a)
	binary.LittleEndian.PutUint32(raw[8:12], 443)
	binary.LittleEndian.PutUint32(raw[12:16], 0)
	binary.LittleEndian.PutUint32(raw[16:20], networkReasonBlocklist)
	binary.LittleEndian.PutUint32(raw[20:24], 7)

	event, err := networkEventFromSample(raw, bundle, handle)
	if err != nil {
		t.Fatalf("networkEventFromSample failed: %v", err)
	}
	if event.EventType != "network.connect" || event.Decision != "block" || event.Reason != "blocklist" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.DstIP != "203.0.113.10" || event.DstPort != 443 || event.MatchedRule != "*.example.com" {
		t.Fatalf("unexpected destination/rule: %+v", event)
	}
}
