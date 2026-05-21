package skills

import (
	"bufio"
	"bytes"
	"strings"
)

// ParseFrontmatter extracts name and description from SKILL.md frontmatter.
// Supports simple YAML-style frontmatter between --- markers.
func ParseFrontmatter(data []byte) (name, description string, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			name = val
		case "description":
			description = val
		}
	}
	return name, description, scanner.Err()
}
