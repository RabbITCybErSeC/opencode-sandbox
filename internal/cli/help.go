package cli

import "fmt"

func runHelp(args []string) error {
	fmt.Println("opencode-sandbox <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  doctor    Check environment readiness")
	fmt.Println("  init      Create project or global config")
	fmt.Println("  run       Run OpenCode in the sandbox")
	fmt.Println("  skills    Manage imported skills")
	fmt.Println("  policy    Test network policy")
	fmt.Println("  image     Pull published images or build from source")
	fmt.Println("  config    Inspect configuration")
	fmt.Println("  uninstall Remove global artifacts and container resources")
	fmt.Println("  help      Show this help")
	fmt.Println()
	fmt.Println("Anything else is forwarded to OpenCode.")
	return nil
}
