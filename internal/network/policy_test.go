package network

import (
	"testing"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func TestEngineAllowlistWins(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{"example.com"},
			Allowlist:     []string{"example.com"},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com")
	if dec.Blocked {
		t.Error("allowlist should win over blocklist")
	}
	if dec.MatchedRule != "example.com" {
		t.Errorf("expected matched allowlist rule, got %q", dec.MatchedRule)
	}
}

func TestEngineBlocklist(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{"*.segment.io"},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("api.segment.io")
	if !dec.Blocked {
		t.Error("expected api.segment.io to be blocked")
	}
	if dec.MatchedRule != "*.segment.io" {
		t.Errorf("expected matched rule *.segment.io, got %q", dec.MatchedRule)
	}
}

func TestEngineWildcardBlocklist(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{"*.example.com"},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})

	cases := []struct {
		host    string
		blocked bool
	}{
		{"api.example.com", true},
		{"foo.bar.example.com", true},
		{"example.com", false},
		{"other.com", false},
	}

	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			dec := eng.Test(c.host)
			if dec.Blocked != c.blocked {
				t.Errorf("Test(%q).Blocked = %v, want %v", c.host, dec.Blocked, c.blocked)
			}
		})
	}
}

func TestEnginePracticalDefaultAllow(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com")
	if dec.Blocked {
		t.Error("practical mode with defaultAction allow should default allow")
	}
	if dec.Note != "default allow" {
		t.Errorf("unexpected note: %q", dec.Note)
	}
}

func TestEngineStrictDefaultAllow(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "strict",
			Backend:       "ebpf",
			DefaultAction: "allow",
			Blocklist:     []string{},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com")
	if dec.Blocked {
		t.Error("strict mode with defaultAction allow should default allow")
	}
	if dec.Note != "default allow" {
		t.Errorf("unexpected note: %q", dec.Note)
	}
}

func TestEngineStrictDefaultDeny(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "strict",
			Backend:       "ebpf",
			DefaultAction: "deny",
			Blocklist:     []string{},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com")
	if !dec.Blocked {
		t.Error("strict mode with defaultAction deny should default deny")
	}
	if dec.Note != "default deny" {
		t.Errorf("unexpected note: %q", dec.Note)
	}
}

func TestEngineStrictAllowlist(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "strict",
			Backend:       "ebpf",
			DefaultAction: "deny",
			Blocklist:     []string{},
			Allowlist:     []string{"safe.example.com"},
			FailClosed:    true,
		},
	})
	dec := eng.Test("safe.example.com")
	if dec.Blocked {
		t.Error("allowlisted domain should be allowed in strict mode")
	}
}

func TestEngineOffMode(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "off",
			Backend:       "ebpf",
			DefaultAction: "deny",
			Blocklist:     []string{},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com")
	if !dec.Blocked {
		t.Error("off mode should block everything")
	}
	if dec.Note != "network off: all outbound denied" {
		t.Errorf("unexpected note: %q", dec.Note)
	}
}

func TestEngineCaseNormalization(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{"EXAMPLE.COM"},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("Example.Com")
	if !dec.Blocked {
		t.Error("matching should be case-insensitive")
	}
}

func TestEngineTrailingDot(t *testing.T) {
	eng := NewEngine(config.EffectiveConfig{
		Network: config.EffectiveNetwork{
			Mode:          "practical",
			Backend:       "proxy",
			DefaultAction: "allow",
			Blocklist:     []string{"example.com"},
			Allowlist:     []string{},
			FailClosed:    true,
		},
	})
	dec := eng.Test("example.com.")
	if !dec.Blocked {
		t.Error("trailing dot should be normalized before matching")
	}
}
