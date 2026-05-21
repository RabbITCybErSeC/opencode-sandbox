package network

import "testing"

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{"EXAMPLE.COM.", "example.com"},
		{"api.example.com", "api.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeDomain(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		domain string
		rule   string
		want   bool
	}{
		{"example.com", "example.com", true},
		{"EXAMPLE.COM", "example.com", true},
		{"example.com.", "example.com", true},
		{"api.example.com", "*.example.com", true},
		{"foo.bar.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"other.com", "example.com", false},
		{"api.example.com", "example.com", false},
		{"segment.io", "*.segment.io", false},
		{"api.segment.io", "*.segment.io", true},
	}
	for _, tt := range tests {
		t.Run(tt.domain+"_"+tt.rule, func(t *testing.T) {
			got := MatchDomain(tt.domain, tt.rule)
			if got != tt.want {
				t.Errorf("MatchDomain(%q, %q) = %v, want %v", tt.domain, tt.rule, got, tt.want)
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"example.com", "example.com", false},
		{"https://example.com/path", "example.com", false},
		{"http://api.example.com:8080/", "api.example.com", false},
		{"  example.com  ", "example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ExtractHostname(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractHostname(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
