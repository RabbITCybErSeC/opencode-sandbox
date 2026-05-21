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
)

// Policy describes the network policy.
type Policy struct {
	Mode          string   `json:"mode"`
	DefaultAction string   `json:"defaultAction"`
	Blocklist     []string `json:"blocklist"`
	Allowlist     []string `json:"allowlist"`
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

	logFile := os.Getenv("POLICY_LOG_FILE")
	if logFile == "" {
		logFile = "/sandbox/logs/network.log"
	}

	logger, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
	Logger *os.File
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

	hostname, _, _ := net.SplitHostPort(host)
	if hostname == "" {
		hostname = host
	}

	decision, rule := p.decide(hostname)
	if decision == "block" {
		p.logBlock(hostname, rule, "CONNECT")
		http.Error(w, "blocked by policy", http.StatusForbidden)
		return
	}

	dest, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer dest.Close()

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

	decision, rule := p.decide(hostname)
	if decision == "block" {
		p.logBlock(hostname, rule, "HTTP")
		http.Error(w, "blocked by policy", http.StatusForbidden)
		return
	}

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

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

func (p *Proxy) logBlock(hostname, rule, method string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("%s blocked=%s rule=%q method=%s mode=%s\n", ts, hostname, rule, method, p.Policy.Mode)
	p.Logger.WriteString(line)
}

func matchDomain(domain, rule string) bool {
	if strings.HasPrefix(rule, "*.") {
		suffix := rule[2:]
		return strings.HasSuffix(domain, "."+suffix)
	}
	return domain == rule
}
