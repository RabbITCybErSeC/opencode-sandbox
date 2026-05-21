package main

import (
	"fmt"
	"os"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
