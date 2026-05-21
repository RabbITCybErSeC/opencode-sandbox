package containercmd

// Argv is the container run command as a string slice.
type Argv []string

// String returns a human-readable representation.
func (a Argv) String() string {
	return "[container run ...]"
}
