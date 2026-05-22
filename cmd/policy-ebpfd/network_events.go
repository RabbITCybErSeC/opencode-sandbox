package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
	"github.com/cilium/ebpf/perf"
)

const (
	networkReasonAllowlist    = uint32(1)
	networkReasonBlocklist    = uint32(2)
	networkReasonDefaultAllow = uint32(3)
	networkReasonDefaultDeny  = uint32(4)
)

type bpfNetworkEvent struct {
	PID      uint32
	DstIP    uint32
	DstPort  uint32
	Decision uint32
	Reason   uint32
	RuleID   uint32
	_        uint32
}

type NetworkEventMonitor struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
	rd     *perf.Reader
	handle *enforcementHandle
	writer *DaemonEventWriter
}

func StartNetworkEventMonitor(bundle *PolicyBundle, handle *enforcementHandle, writer *DaemonEventWriter) (*NetworkEventMonitor, error) {
	if handle == nil || handle.events == nil {
		return nil, fmt.Errorf("network event map is not available")
	}
	rd, err := perf.NewReader(handle.events, os.Getpagesize())
	if err != nil {
		return nil, fmt.Errorf("creating network event reader: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &NetworkEventMonitor{cancel: cancel, rd: rd, handle: handle, writer: writer}
	monitor.wg.Add(1)
	go monitor.run(ctx, bundle)
	return monitor, nil
}

func (m *NetworkEventMonitor) run(ctx context.Context, bundle *PolicyBundle) {
	defer m.wg.Done()
	for {
		record, err := m.rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "network-connect", Reason: "read-error", Error: err.Error()})
				continue
			}
		}
		if record.LostSamples > 0 {
			_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "network-connect", Reason: "lost-samples", LostSamples: record.LostSamples})
		}
		event, err := networkEventFromSample(record.RawSample, bundle, m.handle)
		if err != nil {
			_ = m.writer.WriteAudit(auditlog.Event{EventType: auditlog.EventAuditError, RunID: bundle.RunID, Project: bundle.Project.Name, Backend: "ebpf", Component: "network-connect", Reason: "parse-error", Error: err.Error()})
			continue
		}
		if err := m.writer.WriteAudit(event); err != nil {
			fmt.Fprintf(os.Stderr, "policy-ebpfd: writing network event: %v\n", err)
		}
	}
}

func networkEventFromSample(raw []byte, bundle *PolicyBundle, handle *enforcementHandle) (auditlog.Event, error) {
	if len(raw) < 28 {
		return auditlog.Event{}, fmt.Errorf("short network event sample: %d bytes", len(raw))
	}
	ev := bpfNetworkEvent{
		PID:      binary.LittleEndian.Uint32(raw[0:4]),
		DstIP:    binary.LittleEndian.Uint32(raw[4:8]),
		DstPort:  binary.LittleEndian.Uint32(raw[8:12]),
		Decision: binary.LittleEndian.Uint32(raw[12:16]),
		Reason:   binary.LittleEndian.Uint32(raw[16:20]),
		RuleID:   binary.LittleEndian.Uint32(raw[20:24]),
	}
	decision := "block"
	if ev.Decision != 0 {
		decision = "allow"
	}
	dstIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(dstIP, ev.DstIP)
	port := int(ev.DstPort)
	if port > 65535 {
		port = int(binary.BigEndian.Uint16(raw[8:10]))
	}
	return auditlog.Event{
		EventType:   auditlog.EventNetworkConnect,
		RunID:       bundle.RunID,
		Project:     bundle.Project.Name,
		Backend:     "ebpf",
		Hook:        "cgroup-connect4",
		PID:         int(ev.PID),
		Protocol:    "tcp",
		DstIP:       dstIP.String(),
		DstPort:     port,
		Decision:    decision,
		Reason:      networkReasonText(ev.Reason),
		MatchedRule: handle.ruleName(ev.RuleID),
	}, nil
}

func networkReasonText(reason uint32) string {
	switch reason {
	case networkReasonAllowlist:
		return "allowlist"
	case networkReasonBlocklist:
		return "blocklist"
	case networkReasonDefaultAllow:
		return "default-allow"
	case networkReasonDefaultDeny:
		return "default-deny"
	default:
		return "unknown"
	}
}

func (m *NetworkEventMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.rd != nil {
		_ = m.rd.Close()
	}
	m.wg.Wait()
	return nil
}
