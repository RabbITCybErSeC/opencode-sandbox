package main

import (
	"encoding/binary"
	"fmt"
	"net"

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
	defaultAction uint32
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
	value := uint32(1)
	return h.blockedIPv4.Update(key, value, ebpf.UpdateAny)
}

func (h *enforcementHandle) UpdateAllowed(ip net.IP, rule string) error {
	key, ok := ipv4Key(ip)
	if !ok {
		return nil
	}
	value := uint32(1)
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

	defaultAction := uint32(1)
	if bundle.Network.DefaultAction == "deny" || bundle.Network.Mode == "off" {
		defaultAction = 0
	}
	key := uint32(0)
	if err := settings.Update(key, defaultAction, ebpf.UpdateAny); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		return nil, fmt.Errorf("initializing settings map: %w", err)
	}

	insns := cgroupConnect4Instructions()
	if err := insns.AssociateMap("settings", settings); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		return nil, fmt.Errorf("associating settings map: %w", err)
	}
	if err := insns.AssociateMap("blocked_ipv4", blockedIPv4); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		return nil, fmt.Errorf("associating blocked map: %w", err)
	}
	if err := insns.AssociateMap("allowed_ipv4", allowedIPv4); err != nil {
		settings.Close()
		blockedIPv4.Close()
		allowedIPv4.Close()
		return nil, fmt.Errorf("associating allowed map: %w", err)
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
		return nil, fmt.Errorf("attaching cgroup/connect: %w", err)
	}

	return &enforcementHandle{
		program:       prog,
		link:          l,
		settings:      settings,
		blockedIPv4:   blockedIPv4,
		allowedIPv4:   allowedIPv4,
		defaultAction: defaultAction,
	}, nil
}

func cgroupConnect4Instructions() asm.Instructions {
	return asm.Instructions{
		asm.LoadMem(asm.R6, asm.R1, 4, asm.Word),
		asm.StoreMem(asm.RFP, -4, asm.R6, asm.Word),

		asm.LoadMapPtr(asm.R1, 0).WithReference("allowed_ipv4"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -4),
		asm.FnMapLookupElem.Call(),
		asm.JNE.Imm(asm.R0, 0, "allow"),

		asm.LoadMapPtr(asm.R1, 0).WithReference("blocked_ipv4"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -4),
		asm.FnMapLookupElem.Call(),
		asm.JNE.Imm(asm.R0, 0, "deny"),

		asm.StoreImm(asm.RFP, -8, 0, asm.Word),
		asm.LoadMapPtr(asm.R1, 0).WithReference("settings"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -8),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "deny"),
		asm.LoadMem(asm.R0, asm.R0, 0, asm.Word),
		asm.JNE.Imm(asm.R0, 0, "allow"),

		asm.Mov.Imm(asm.R0, 0).WithSymbol("deny"),
		asm.Return(),
		asm.Mov.Imm(asm.R0, 1).WithSymbol("allow"),
		asm.Return(),
	}
}

func ipv4Key(ip net.IP) (uint32, bool) {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(ip4), true
}
