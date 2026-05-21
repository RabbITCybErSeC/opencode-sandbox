package main

import (
	"fmt"
	"net"
)

// stubMapManager logs map operations without a real eBPF runtime.
// It is used on non-Linux systems or when eBPF loading fails.
type stubMapManager struct{}

func newStubMapManager() *stubMapManager {
	return &stubMapManager{}
}

func (s *stubMapManager) UpdateBlocked(ip net.IP, rule string) error {
	fmt.Printf("stubMapManager: UpdateBlocked ip=%s rule=%s\n", ip, rule)
	return nil
}

func (s *stubMapManager) UpdateAllowed(ip net.IP, rule string) error {
	fmt.Printf("stubMapManager: UpdateAllowed ip=%s rule=%s\n", ip, rule)
	return nil
}

func (s *stubMapManager) RemoveBlocked(ip net.IP) error {
	fmt.Printf("stubMapManager: RemoveBlocked ip=%s\n", ip)
	return nil
}

func (s *stubMapManager) RemoveAllowed(ip net.IP) error {
	fmt.Printf("stubMapManager: RemoveAllowed ip=%s\n", ip)
	return nil
}
