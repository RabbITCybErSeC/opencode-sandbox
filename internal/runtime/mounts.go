package runtime

// Mount describes a bind mount.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// DefaultMounts returns the default set of mounts.
func DefaultMounts(projectPath string) []Mount {
	return []Mount{
		{Source: projectPath, Target: "/workspace", ReadOnly: false},
	}
}
