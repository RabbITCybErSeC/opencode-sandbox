package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RabbITCybErSeC/opencode-sandbox/internal/config"
)

func runInit(args []string) error {
	global := false
	force := false
	project := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--global":
			global = true
		case "--force":
			force = true
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		}
	}

	if global {
		return initGlobal(force)
	}
	return initProject(project, force)
}

func initGlobal(force bool) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("getting config dir: %w", err)
	}
	path := configDir + "/opencode-sandbox/config.yaml"

	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("global config already exists at %s; use --force to overwrite", path)
	}

	if err := os.MkdirAll(configDir+"/opencode-sandbox", 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	comments := detectExistingConfigs("")
	content := globalConfigYAML(comments)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing global config: %w", err)
	}
	fmt.Printf("Created global config at %s\n", path)
	return nil
}

func initProject(project string, force bool) error {
	if project == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		project = wd
	}

	path := project + "/.opencode-sandbox.yaml"
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("project config already exists at %s; use --force to overwrite", path)
	}

	comments := detectExistingConfigs(project)
	content := projectConfigYAML(comments)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing project config: %w", err)
	}
	fmt.Printf("Created project config at %s\n", path)
	return nil
}

func detectExistingConfigs(project string) string {
	var notes []string
	if project != "" {
		for _, p := range []string{
			filepath.Join(project, "opencode.json"),
			filepath.Join(project, ".opencode", "skills"),
			filepath.Join(project, ".agents", "skills"),
			filepath.Join(project, ".claude", "skills"),
		} {
			if _, err := os.Stat(p); err == nil {
				notes = append(notes, fmt.Sprintf("# Found: %s", p))
			}
		}
	}
	if len(notes) > 0 {
		return strings.Join(notes, "\n") + "\n"
	}
	return ""
}

func globalConfigYAML(comments string) string {
	if comments != "" {
		comments = "\n" + comments
	}
	return fmt.Sprintf(`version: 1%s
image:
  name: %s

opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false

skills:
  importedDir: ~/.config/opencode-sandbox/skills
  include:
    - "*"
  exclude: []

network:
  mode: practical
  ebpf:
    initImage: %s
  blocklist: []
  allowlist: []
  proxyPort: 18080
  dnsPort: 15353
  failClosed: true
  # Allow the container to reach services on the macOS host (e.g., MCP servers).
  # Requires: sudo container system dns create host.container.internal --localhost 203.0.113.113
  localhostAccess:
    enabled: false
    ip: 203.0.113.113
    domain: host.container.internal

audit:
  commands:
    enabled: true
    backend: ebpf
    failClosed: false
    logArgs: full
    maxArgs: 64
    maxArgBytes: 16384
    includeExecutables: []
    excludeExecutables: []
    includeCwd: []
    excludeCwd: []
    mirrorProjectEvents: false
    eventLog: ~/.local/state/opencode-sandbox/runs

resources:
  cpus: 4
  memory: 4g
`, comments, config.DefaultImageName, config.DefaultStrictInitImage)
}

func projectConfigYAML(comments string) string {
	if comments != "" {
		comments = "\n" + comments
	}
	return fmt.Sprintf(`version: 1%s
image:
  name: %s

project:
  target: /workspace
  readonly: false

opencode:
  mountHostConfig: true
  mountHostData: true
  generatedConfig: true
  autoupdate: false

skills:
  importedDir: .opencode-sandbox/skills
  include:
    - "*"
  exclude: []

network:
  inheritGlobal: true
  mode: practical
  ebpf:
    initImage: %s
  blocklist: []
  allowlist: []
  # Allow the container to reach services on the macOS host (e.g., MCP servers).
  # Requires: sudo container system dns create host.container.internal --localhost 203.0.113.113
  localhostAccess:
    enabled: false
    ip: 203.0.113.113
    domain: host.container.internal

audit:
  commands:
    enabled: true
    backend: ebpf
    failClosed: false
    logArgs: full
    maxArgs: 64
    maxArgBytes: 16384
    includeExecutables: []
    excludeExecutables: []
    includeCwd: []
    excludeCwd: []
    mirrorProjectEvents: false
    eventLog: ~/.local/state/opencode-sandbox/runs

resources:
  cpus: 4
  memory: 4g

container:
  namePrefix: opencode-sandbox
  remove: true
`, comments, config.DefaultImageName, config.DefaultStrictInitImage)
}
