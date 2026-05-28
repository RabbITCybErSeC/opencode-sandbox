package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
)

type enforcementHandle struct {
	program       *ebpf.Program
	link          link.Link
	settings      *ebpf.Map
	blockedIPv4   *ebpf.Map
	allowedIPv4   *ebpf.Map
	events        *ebpf.Map
	defaultAction uint32
	mu            sync.Mutex
	nextRuleID    uint32
	rules         map[uint32]string
}

func (h *enforcementHandle) Close() error {
	var firstErr error
	for _, closeFn := range []func() error{
		func() error {
			if h.link != nil {
				return h.link.Close()
			}
			return nil
		},
		func() error {
			if h.program != nil {
				return h.program.Close()
			}
			return nil
		},
		func() error {
			if h.settings != nil {
				return h.settings.Close()
			}
			return nil
		},
		func() error {
			if h.blockedIPv4 != nil {
				return h.blockedIPv4.Close()
			}
			return nil
		},
		func() error {
			if h.allowedIPv4 != nil {
				return h.allowedIPv4.Close()
			}
			return nil
		},
		func() error {
			if h.events != nil {
				return h.events.Close()
			}
			return nil
		},
	} {
		if err := closeFn(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *enforcementHandle) UpdateBlocked(ip net.IP, rule string) error {
	key, ok := ipv4Key(ip)
	if !ok {
		return nil
	}
	value := h.ruleID(rule)
	return h.blockedIPv4.Update(key, value, ebpf.UpdateAny)
}

func (h *enforcementHandle) UpdateAllowed(ip net.IP, rule string) error {
	key, ok := ipv4Key(ip)
	if !ok {
		return nil
	}
	value := h.ruleID(rule)
	return h.allowedIPv4.Update(key, value, ebpf.UpdateAny)
}

func (h *enforcementHandle) RemoveBlocked(ip net.IP) error {
	key, ok := ipv4Key(ip)
	if !ok {
		return nil
	}
	return h.blockedIPv4.Delete(key)
}

func (h *enforcementHandle) RemoveAllowed(ip net.IP) error {
	key, ok := ipv4Key(ip)
	if !ok {
		return nil
	}
	return h.allowedIPv4.Delete(key)
}

func (h *enforcementHandle) ruleID(rule string) uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rules == nil {
		h.rules = map[uint32]string{}
	}
	h.nextRuleID++
	if h.nextRuleID == 0 {
		h.nextRuleID = 1
	}
	h.rules[h.nextRuleID] = rule
	return h.nextRuleID
}

func (h *enforcementHandle) ruleName(id uint32) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rules[id]
}

// attachCgroupConnect loads an IPv4 cgroup/connect eBPF program and attaches
// it for the daemon lifetime. The program uses exact-IP maps that are updated
// by the resolver. Allowlist wins, then blocklist, then defaultAction.
func attachCgroupConnect(cgroupPath string, bundle *PolicyBundle) (*enforcementHandle, error) {
	settings, err := ebpf.NewMap(&ebpf.MapSpec{
		Name:       "settings",
		Type:       ebpf.Array,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("creating settings map: %w", err)
	}

	blockedIPv4, err := ebpf.NewMap(&ebpf.MapSpec{
		Name:       "blocked_ipv4",
		Type:       ebpf.Hash,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 4096,
	})
	if err != nil {
		settings.Close()
		return nil, fmt.Errorf("creating blocked IPv4 map: %w", err)
	}

	allowedIPv4, err := ebpf.NewMap(&ebpf.MapSpec{
		Name:       "allowed_ipv4",
		Type:       ebpf.Hash,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 4096,
	})
	if err != nil {
		settings.Close()
		blockedIPv4.Close()
		return nil, fmt.Errorf("creating allowed IPv4 map: %w", err)
	}

	events, err := ebpf.NewMap(&ebpf.MapSpec{
		Type: ebpf.PerfEventArray,
		Name: "network_events",
	})
	if err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		return nil, fmt.Errorf("creating network event map: %w", err)
	}

	defaultAction := uint32(1)
	if bundle.Network.DefaultAction == "deny" || bundle.Network.Mode == "off" {
		defaultAction = 0
	}
	key := uint32(0)
	if err := settings.Update(key, defaultAction, ebpf.UpdateAny); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("initializing settings map: %w", err)
	}

	insns := cgroupConnect4Instructions()
	if err := insns.AssociateMap("settings", settings); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("associating settings map: %w", err)
	}
	if err := insns.AssociateMap("blocked_ipv4", blockedIPv4); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("associating blocked map: %w", err)
	}
	if err := insns.AssociateMap("allowed_ipv4", allowedIPv4); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("associating allowed map: %w", err)
	}
	if err := insns.AssociateMap("network_events", events); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("associating network event map: %w", err)
	}

	progSpec := &ebpf.ProgramSpec{
		Name:         "cgroup_connect4",
		Type:         ebpf.CGroupSockAddr,
		License:      "GPL",
		Instructions: insns,
	}

	prog, err := ebpf.NewProgram(progSpec)
	if err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("loading cgroup/connect program: %w", err)
	}

	l, err := link.AttachCgroup(link.CgroupOptions{
		Path:    cgroupPath,
		Attach:  ebpf.AttachCGroupInet4Connect,
		Program: prog,
	})
	if err != nil {
		prog.Close()
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		events.Close()
		return nil, fmt.Errorf("attaching cgroup/connect: %w", err)
	}

	return &enforcementHandle{
		program:       prog,
		link:          l,
		settings:      settings,
		blockedIPv4:   blockedIPv4,
		allowedIPv4:   allowedIPv4,
		events:        events,
		defaultAction: defaultAction,
		rules:         map[uint32]string{},
	}, nil
}

func cgroupConnect4Instructions() asm.Instructions {
	insns := asm.Instructions{
		asm.Mov.Reg(asm.R9, asm.R1),
		asm.LoadMem(asm.R6, asm.R1, 4, asm.Word),
		asm.StoreMem(asm.RFP, -4, asm.R6, asm.Word),

		asm.LoadMapPtr(asm.R1, 0).WithReference("allowed_ipv4"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -4),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "check_blocked"),
		asm.LoadMem(asm.R7, asm.R0, 0, asm.Word),
		asm.Mov.Imm(asm.R8, 1),
		asm.Ja.Label("allow"),

		asm.Mov.Imm(asm.R0, 0).WithSymbol("check_blocked"),
		asm.LoadMapPtr(asm.R1, 0).WithReference("blocked_ipv4"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -4),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "check_default"),
		asm.LoadMem(asm.R7, asm.R0, 0, asm.Word),
		asm.Mov.Imm(asm.R8, 2),
		asm.Ja.Label("deny"),

		asm.Mov.Imm(asm.R0, 0).WithSymbol("check_default"),
		asm.StoreImm(asm.RFP, -8, 0, asm.Word),
		asm.LoadMapPtr(asm.R1, 0).WithReference("settings"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -8),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "default_deny"),
		asm.LoadMem(asm.R0, asm.R0, 0, asm.Word),
		asm.JEq.Imm(asm.R0, 0, "default_deny"),
		asm.Mov.Imm(asm.R7, 0),
		asm.Mov.Imm(asm.R8, 3),
		asm.Ja.Label("allow"),

		asm.Mov.Imm(asm.R7, 0).WithSymbol("default_deny"),
		asm.Mov.Imm(asm.R8, 4),
		asm.Ja.Label("deny"),

		asm.Mov.Imm(asm.R0, 0).WithSymbol("deny"),
	}
	insns = append(insns, networkEventOutputInstructions(0)...)
	insns = append(insns,
		asm.Return(),
		asm.Mov.Imm(asm.R0, 1).WithSymbol("allow"),
	)
	insns = append(insns, networkEventOutputInstructions(1)...)
	insns = append(insns,
		asm.Return(),
	)
	return insns
}

func networkEventOutputInstructions(decision int32) asm.Instructions {
	return asm.Instructions{
		asm.StoreMem(asm.RFP, -32, asm.R6, asm.Word),
		asm.FnGetCurrentPidTgid.Call(),
		asm.RSh.Imm(asm.R0, 32),
		asm.StoreMem(asm.RFP, -36, asm.R0, asm.Word),
		asm.LoadMem(asm.R0, asm.R9, 20, asm.Word),
		asm.StoreMem(asm.RFP, -28, asm.R0, asm.Word),
		asm.Mov.Imm(asm.R0, decision),
		asm.StoreMem(asm.RFP, -24, asm.R0, asm.Word),
		asm.StoreMem(asm.RFP, -20, asm.R8, asm.Word),
		asm.StoreMem(asm.RFP, -16, asm.R7, asm.Word),
		asm.Mov.Imm(asm.R0, 0),
		asm.StoreMem(asm.RFP, -12, asm.R0, asm.Word),
		asm.StoreMem(asm.RFP, -8, asm.R0, asm.Word),
		asm.Mov.Reg(asm.R1, asm.R9),
		asm.LoadMapPtr(asm.R2, 0).WithReference("network_events"),
		asm.LoadImm(asm.R3, 0xffffffff, asm.DWord),
		asm.Mov.Reg(asm.R4, asm.RFP),
		asm.Add.Imm(asm.R4, -36),
		asm.Mov.Imm(asm.R5, 28),
		asm.FnPerfEventOutput.Call(),
		asm.Mov.Imm(asm.R0, decision),
	}
}

func ipv4Key(ip net.IP) (uint32, bool) {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(ip4), true
}
