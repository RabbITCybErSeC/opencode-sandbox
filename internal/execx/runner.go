package execx

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Runner abstracts subprocess execution.
type Runner interface {
	Run(cmd *exec.Cmd) error
}

// RealRunner executes commands using os/exec.
type RealRunner struct{}

func (r *RealRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

// ExitCodeError preserves a child process exit code without exiting this process.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// ExitCode extracts a preserved child exit code from err.
func ExitCode(err error) (int, bool) {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code, true
	}
	return 0, false
}

// RunContainer executes a container command with proper I/O forwarding and
// exit code propagation.
func RunContainer(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return &ExitCodeError{Code: status.ExitStatus(), Err: err}
			}
		}
		return err
	}
	return nil
}
