package execx

import (
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
				os.Exit(status.ExitStatus())
			}
		}
		return err
	}
	return nil
}
