package execx

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestFakeRunnerRecordsArgv(t *testing.T) {
	f := NewFakeRunner()
	cmd := exec.Command("container", "run", "--rm", "image:latest")
	if err := f.Run(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls))
	}
	want := []string{"container", "run", "--rm", "image:latest"}
	got := f.Calls[0].Argv
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestFakeRunnerConfiguredError(t *testing.T) {
	f := NewFakeRunner()
	f.Errors["container run --rm image:latest"] = fmt.Errorf("exit status 1")
	cmd := exec.Command("container", "run", "--rm", "image:latest")
	if err := f.Run(cmd); err == nil {
		t.Fatal("expected configured error")
	}
}
