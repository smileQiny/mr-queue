package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildExternalMergeDoesNotRequireMaintainerToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")
	writeFile(t, configPath, `
local:
  path: "."
workflow:
  commit_range: "main..HEAD"
  merge_method: "external"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
  reviewer:
    token_env: "GITCODE_REVIEWER_TOKEN"
`)
	writeFile(t, envPath, `
GITCODE_SUBMITTER_TOKEN=sub-token
GITCODE_REVIEWER_TOKEN=review-token
`)

	if _, err := Build(configPath, envPath, filepath.Join(dir, "state.json")); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
}

func TestBuildInternalMergeRequiresMaintainerToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")
	writeFile(t, configPath, `
local:
  path: "."
workflow:
  commit_range: "main..HEAD"
  merge_method: "squash"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
  reviewer:
    token_env: "GITCODE_REVIEWER_TOKEN"
`)
	writeFile(t, envPath, `
GITCODE_SUBMITTER_TOKEN=sub-token
GITCODE_REVIEWER_TOKEN=review-token
`)

	_, err := Build(configPath, envPath, filepath.Join(dir, "state.json"))
	if err == nil {
		t.Fatal("expected missing maintainer token error")
	}
	if !strings.Contains(err.Error(), "maintainer token is required") {
		t.Fatalf("error = %v", err)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
