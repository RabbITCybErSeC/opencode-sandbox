package main

import (
	"fmt"
	"os"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/cli"
	"github.com/RabbITCybErSeC/opencode-sandbox/internal/execx"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if code, ok := execx.ExitCode(err); ok {
			os.Exit(code)
		}
		os.Exit(1)
	}
}
