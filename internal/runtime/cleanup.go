package runtime

// Cleanup handles removal of staging directories.
type Cleanup struct {
	Keep bool
}

// NewCleanup creates a cleanup helper.
func NewCleanup(keep bool) *Cleanup {
	return &Cleanup{Keep: keep}
}

// Remove staging directory unless Keep is set.
func (c *Cleanup) Remove(dir string) error {
	if c.Keep {
		return nil
	}
	return SafeRemoveAll(dir)
}
