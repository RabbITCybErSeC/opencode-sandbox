package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	auditlog "github.com/RabbITCybErSeC/opencode-sandbox/internal/audit"
)

// Policy describes the network policy.
type Policy struct {
	Mode          string   `json:"mode"`
	DefaultAction string   `json:"defaultAction"`
	Blocklist     []string `json:"blocklist"`
	Allowlist     []string `json:"allowlist"`
	RunID         string   `json:"runId"`
	Project       struct {
		Name string `json:"name"`
	} `json:"project"`
	Network struct {
		Backend string `json:"backend"`
	} `json:"network"`
	Audit struct {
		Events struct {
			HostJsonl           string                  `json:"hostJsonl"`
			ProjectMirrorJsonl  string                  `json:"projectMirrorJsonl"`
			MirrorProjectEvents bool                    `json:"mirrorProjectEvents"`
			Rotation            auditlog.RotationConfig `json:"rotation"`
		} `json:"events"`
	} `json:"audit"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: policy-proxy <policy.json>")
		os.Exit(1)
	}

	policyPath := os.Args[1]
	data, err := os.ReadFile(policyPath)
	if err != nil {
		log.Fatalf("reading policy: %v", err)
	}

	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		log.Fatalf("parsing policy: %v", err)
	}

	proxyPort := os.Getenv("POLICY_PROXY_PORT")
	if proxyPort == "" {
		proxyPort = "18080"
	}

	hostLog := policy.Audit.Events.HostJsonl
	if hostLog == "" {
		hostLog = os.Getenv("POLICY_LOG_FILE")
	}
	if hostLog == "" {
		hostLog = "/sandbox/logs/" + auditlog.DefaultFileName
	}
	rotation := policy.Audit.Events.Rotation
	if rotation.MaxBytes == 0 {
		rotation.MaxBytes = auditlog.DefaultRotationMaxBytes
	}
	if rotation.MaxFiles == 0 {
		rotation.MaxFiles = auditlog.DefaultRotationMaxFiles
	}
	logger, err := auditlog.NewWriter(
		hostLog,
		policy.Audit.Events.ProjectMirrorJsonl,
		policy.Audit.Events.MirrorProjectEvents,
		rotation,
	)
	if err != nil {
		log.Fatalf("opening log file: %v", err)
	}
	defer logger.Close()

	proxy := &Proxy{
		Policy: policy,
		Logger: logger,
	}

	addr := ":" + proxyPort
	log.Printf("policy proxy starting on %s", addr)
	server := &http.Server{
		Addr:    addr,
		Handler: proxy,
	}
	log.Fatal(server.ListenAndServe())
}

// Proxy implements an HTTP CONNECT proxy with policy enforcement.
type Proxy struct {
	Policy Policy
	Logger *auditlog.Writer
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}

	hostname, port := splitHostPort(host)

	decision, rule := p.decide(hostname)
	if decision == "block" {
		p.logNetwork(hostname, port, "CONNECT", decision, "policy-block", rule, "")
		http.Error(w, "blocked by policy", http.StatusForbidden)
		return
	}

	dest, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		p.logNetwork(hostname, port, "CONNECT", "error", "upstream-error", rule, err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer dest.Close()
	p.logNetwork(hostname, port, "CONNECT", decision, "policy-allow", rule, "")

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer client.Close()

	go func() {
		io.Copy(dest, client)
	}()
	io.Copy(client, dest)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Hostname()
	if hostname == "" {
		hostname = r.Host
	}
	hostname, port := splitHostPort(hostname)
	if port == 0 {
		port = defaultPort(r.URL.Scheme)
	}

	decision, rule := p.decide(hostname)
	if decision == "block" {
		p.logNetwork(hostname, port, "HTTP", decision, "policy-block", rule, "")
		http.Error(w, "blocked by policy", http.StatusForbidden)
		return
	}

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	resp, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		p.logNetwork(hostname, port, "HTTP", "error", "upstream-error", rule, err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	p.logNetwork(hostname, port, "HTTP", decision, "policy-allow", rule, "")

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *Proxy) decide(hostname string) (string, string) {
	host := strings.ToLower(strings.TrimSuffix(hostname, "."))

	// Allowlist wins.
	for _, rule := range p.Policy.Allowlist {
		if matchDomain(host, rule) {
			return "allow", rule
		}
	}

	// Blocklist blocks.
	for _, rule := range p.Policy.Blocklist {
		if matchDomain(host, rule) {
			return "block", rule
		}
	}

	if p.Policy.DefaultAction == "deny" || p.Policy.Mode == "off" {
		return "block", "default-deny"
	}
	return "allow", ""
}

func (p *Proxy) logNetwork(hostname string, port int, method, decision, reason, rule, errText string) {
	if p.Logger == nil {
		return
	}
	backend := p.Policy.Network.Backend
	if backend == "" {
		backend = "proxy"
	}
	_ = p.Logger.Write(auditlog.Event{
		EventType:   auditlog.EventNetworkConnect,
		RunID:       p.Policy.RunID,
		Project:     p.Policy.Project.Name,
		Backend:     backend,
		Method:      method,
		Protocol:    "tcp",
		Host:        hostname,
		DstPort:     port,
		Decision:    decision,
		Reason:      reason,
		MatchedRule: rule,
		Error:       errText,
	})
}

func matchDomain(domain, rule string) bool {
	if strings.HasPrefix(rule, "*.") {
		suffix := rule[2:]
		return strings.HasSuffix(domain, "."+suffix)
	}
	return domain == rule
}

func splitHostPort(hostport string) (string, int) {
	hostname, portText, err := net.SplitHostPort(hostport)
	if err != nil {
		return strings.Trim(hostport, "[]"), 0
	}
	port, _ := net.LookupPort("tcp", portText)
	return hostname, port
}

func defaultPort(scheme string) int {
	switch scheme {
	case "https":
		return 443
	case "http":
		return 80
	default:
		return 0
	}
}
