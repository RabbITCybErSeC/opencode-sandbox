package cli

var wrapperCommands = map[string]bool{
	"doctor":    true,
	"init":      true,
	"run":       true,
	"skills":    true,
	"policy":    true,
	"image":     true,
	"config":    true,
	"uninstall": true,
	"help":      true,
}

func Execute(args []string) error {
	if len(args) == 0 {
		return runDefault(args)
	}

	first := args[0]

	// Root-level --help and -h belong to OpenCode, not the wrapper.
	if first == "--help" || first == "-h" {
		return runDefault(args)
	}

	if isWrapperCommand(first) {
		switch first {
		case "doctor":
			return runDoctor(args[1:])
		case "init":
			return runInit(args[1:])
		case "run":
			return runRun(args[1:])
		case "skills":
			return runSkills(args[1:])
		case "policy":
			return runPolicy(args[1:])
		case "image":
			return runImage(args[1:])
		case "config":
			return runConfig(args[1:])
		case "uninstall":
			return runUninstall(args[1:])
		case "help":
			return runHelp(args[1:])
		}
	}

	return runDefault(args)
}

func isWrapperCommand(name string) bool {
	return wrapperCommands[name]
}

func runDefault(args []string) error {
	return runRun(args)
}
