package network

import "github.com/RabbITCybErSeC/opencode-sandbox/internal/config"

// Decision describes the result of a policy evaluation.
type Decision struct {
	Blocked     bool   `json:"blocked"`
	MatchedRule string `json:"matchedRule,omitempty"`
	Mode        string `json:"mode"`
	Note        string `json:"note,omitempty"`
}

// Engine holds compiled policy rules.
type Engine struct {
	Mode          string
	Backend       string
	DefaultAction string
	Blocklist     []string
	Allowlist     []string
	FailClosed    bool
}

// NewEngine builds a policy engine from effective config.
func NewEngine(cfg config.EffectiveConfig) *Engine {
	return &Engine{
		Mode:          cfg.Network.Mode,
		Backend:       cfg.Network.Backend,
		DefaultAction: cfg.Network.DefaultAction,
		Blocklist:     cfg.Network.Blocklist,
		Allowlist:     cfg.Network.Allowlist,
		FailClosed:    cfg.Network.FailClosed,
	}
}

// Test evaluates a single hostname against the policy.
func (e *Engine) Test(hostname string) Decision {
	host := NormalizeDomain(hostname)

	// 1. Check allowlist first (allowlist wins)
	for _, rule := range e.Allowlist {
		if MatchDomain(host, rule) {
			return Decision{
				Blocked:     false,
				MatchedRule: rule,
				Mode:        e.Mode,
				Note:        "matched allowlist rule",
			}
		}
	}

	// 2. Check blocklist
	for _, rule := range e.Blocklist {
		if MatchDomain(host, rule) {
			return Decision{
				Blocked:     true,
				MatchedRule: rule,
				Mode:        e.Mode,
				Note:        e.blockNote(),
			}
		}
	}

	// 3. Default based on defaultAction and mode
	blocked := false
	note := "default allow"
	if e.DefaultAction == "deny" || e.Mode == "off" {
		blocked = true
		if e.Mode == "off" {
			note = "network off: all outbound denied"
		} else {
			note = "default deny"
		}
	}

	return Decision{
		Blocked: blocked,
		Mode:    e.Mode,
		Note:    note,
	}
}

func (e *Engine) blockNote() string {
	switch e.Mode {
	case "strict":
		return "strict mode: blocked by policy"
	case "off":
		return "network off: blocked"
	default:
		return "practical mode blocks normal DNS/proxy traffic but cannot guarantee direct-IP blocking"
	}
}
