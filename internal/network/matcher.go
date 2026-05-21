package network

import (
	"net/url"
	"strings"
)

// NormalizeDomain lowercases and strips trailing dots.
func NormalizeDomain(domain string) string {
	domain = strings.ToLower(domain)
	domain = strings.TrimSuffix(domain, ".")
	return domain
}

// MatchDomain checks if a normalized domain matches the given rule.
// Rules:
//   - Exact: "example.com" matches "example.com"
//   - Wildcard: "*.example.com" matches "api.example.com" and
//     "foo.bar.example.com", but not "example.com"
func MatchDomain(domain, rule string) bool {
	domain = NormalizeDomain(domain)
	rule = NormalizeDomain(rule)

	if strings.HasPrefix(rule, "*.") {
		suffix := rule[2:]
		return strings.HasSuffix(domain, "."+suffix)
	}
	return domain == rule
}

// ExtractHostname parses a URL or plain domain and returns the hostname.
func ExtractHostname(input string) (string, error) {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}
	return input, nil
}
