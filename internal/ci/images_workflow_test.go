package ci_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type imagesWorkflow struct {
	Jobs map[string]imagesJob `yaml:"jobs"`
}

type imagesJob struct {
	Strategy imagesStrategy `yaml:"strategy"`
}

type imagesStrategy struct {
	Matrix imagesMatrix `yaml:"matrix"`
}

type imagesMatrix struct {
	Include []publishedImage `yaml:"include"`
}

type publishedImage struct {
	Image     string `yaml:"image"`
	File      string `yaml:"file"`
	Platforms string `yaml:"platforms"`
}

func TestPublishImagesWorkflowUsesSupportedPlatforms(t *testing.T) {
	var workflow imagesWorkflow
	if err := yaml.Unmarshal(readWorkflow(t), &workflow); err != nil {
		t.Fatalf("parse images workflow: %v", err)
	}

	images := map[string]publishedImage{}
	for _, image := range workflow.Jobs["publish"].Strategy.Matrix.Include {
		images[image.Image] = image
	}

	runtimeImage := images["opencode-sandbox"]
	if runtimeImage.Platforms != "linux/amd64,linux/arm64" {
		t.Fatalf("runtime image platforms = %q, want linux/amd64,linux/arm64", runtimeImage.Platforms)
	}

	initImage := images["opencode-sandbox-init"]
	if initImage.File != "Containerfile.init" {
		t.Fatalf("init image file = %q, want Containerfile.init", initImage.File)
	}
	if strings.Contains(initImage.Platforms, "linux/amd64") {
		t.Fatalf("init image platforms include unsupported linux/amd64: %q", initImage.Platforms)
	}
	if initImage.Platforms != "linux/arm64" {
		t.Fatalf("init image platforms = %q, want linux/arm64", initImage.Platforms)
	}
}

func TestStrictInitContainerfileCopiesAuditPackage(t *testing.T) {
	containerfile := string(readRepoFile(t, "Containerfile.init"))
	if !strings.Contains(containerfile, "COPY internal/audit ./internal/audit") {
		t.Fatal("Containerfile.init must copy internal/audit for the policy-ebpfd build")
	}
}

func readWorkflow(t *testing.T) []byte {
	t.Helper()
	return readRepoFile(t, ".github", "workflows", "images.yml")
}

func readRepoFile(t *testing.T, path ...string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	data, err := os.ReadFile(filepath.Join(append([]string{root}, path...)...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
