package main

import (
	"os"
	"testing"
)

func TestPolicyPathFromEnvUsesDefault(t *testing.T) {
	got := policyPathFromEnv(func(string) string { return "" })
	if got != defaultPolicyPath {
		t.Fatalf("policyPathFromEnv() = %q, want %q", got, defaultPolicyPath)
	}
}

func TestPolicyPathFromEnvUsesOverride(t *testing.T) {
	got := policyPathFromEnv(func(string) string { return "/custom/policy.json" })
	if got != "/custom/policy.json" {
		t.Fatalf("policyPathFromEnv() = %q, want override", got)
	}
}

func TestPolicyFileAvailable(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "policy-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if !policyFileAvailable(os.Stat, file.Name()) {
		t.Fatal("expected policy file to be available")
	}
	if policyFileAvailable(os.Stat, t.TempDir()) {
		t.Fatal("directory should not count as an available policy file")
	}
	if policyFileAvailable(os.Stat, file.Name()+".missing") {
		t.Fatal("missing policy file should not be available")
	}
}
