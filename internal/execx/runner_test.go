package execx

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeExtractsWrappedExitCode(t *testing.T) {
	err := fmt.Errorf("running container: %w", &ExitCodeError{Code: 7, Err: errors.New("exit status 7")})
	code, ok := ExitCode(err)
	if !ok {
		t.Fatal("expected exit code")
	}
	if code != 7 {
		t.Fatalf("expected exit code 7, got %d", code)
	}
}
