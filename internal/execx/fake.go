package execx

import "os/exec"

// FakeRunner records commands for testing.
type FakeRunner struct {
	Calls   []FakeCall
	Outputs map[string][]byte
	Errors  map[string]error
}

// FakeCall records a single invocation.
type FakeCall struct {
	Argv []string
}

func NewFakeRunner() *FakeRunner {
	return &FakeRunner{
		Calls:   []FakeCall{},
		Outputs: map[string][]byte{},
		Errors:  map[string]error{},
	}
}

func (f *FakeRunner) Run(cmd *exec.Cmd) error {
	f.Calls = append(f.Calls, FakeCall{Argv: cmd.Args})
	key := callKey(cmd.Args)
	if err, ok := f.Errors[key]; ok {
		return err
	}
	return nil
}

func callKey(args []string) string {
	if len(args) == 0 {
		return ""
	}
	result := args[0]
	for _, a := range args[1:] {
		result += " " + a
	}
	return result
}
