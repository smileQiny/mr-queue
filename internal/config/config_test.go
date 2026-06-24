package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsYAMLAndDotenvTokens(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
source:
  local_path: "."
  base_ref: "main"
  remote: "origin"
  branch_prefix: "mr-queue"
  head_namespace: "submitter"
target:
  owner: "community"
  repo: "project"
  branch: "master"
workflow:
  commit_range: "main..HEAD"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
  reviewer:
    token_env: "GITCODE_REVIEWER_TOKEN"
  maintainer:
    token_env: "GITCODE_MAINTAINER_TOKEN"
`)
	writeFile(t, envPath, `
GITCODE_SUBMITTER_TOKEN=sub-token
GITCODE_REVIEWER_TOKEN=review-token
GITCODE_MAINTAINER_TOKEN=merge-token
`)

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Auth.Submitter.Token != "sub-token" {
		t.Fatalf("submitter token = %q", cfg.Auth.Submitter.Token)
	}
	if cfg.Target.Owner != "community" {
		t.Fatalf("target owner = %q", cfg.Target.Owner)
	}
	safe := cfg.Safe()
	if strings.Contains(safe, "sub-token") {
		t.Fatalf("Safe leaked token: %s", safe)
	}
	if !strings.Contains(safe, "GITCODE_SUBMITTER_TOKEN") {
		t.Fatalf("Safe missing token env name: %s", safe)
	}
}

func TestLoadReadsQueuePrivateCommunityWorkflow(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
queue:
  remote: "private"
  branch: "queue"
  base_ref: "community/master"
private:
  remote: "private"
  branch_prefix: "mr-queue"
  head_namespace: "submitter"
community:
  remote: "community"
  owner: "community"
  repo: "project"
  branch: "master"
workflow:
  merge_method: "squash"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
  reviewer:
    token_env: "GITCODE_REVIEWER_TOKEN"
  maintainer:
    token_env: "GITCODE_MAINTAINER_TOKEN"
`)
	writeFile(t, envPath, `
GITCODE_SUBMITTER_TOKEN=sub-token
GITCODE_REVIEWER_TOKEN=review-token
GITCODE_MAINTAINER_TOKEN=merge-token
`)

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Queue.Remote != "private" || cfg.Queue.Branch != "queue" {
		t.Fatalf("queue = %#v", cfg.Queue)
	}
	if cfg.Private.BranchPrefix != "mr-queue" {
		t.Fatalf("private = %#v", cfg.Private)
	}
	if cfg.Community.Remote != "community" || cfg.Community.Branch != "master" {
		t.Fatalf("community = %#v", cfg.Community)
	}
	if cfg.Source.Remote != "private" || cfg.Source.BranchPrefix != "mr-queue" {
		t.Fatalf("legacy source was not derived from private: %#v", cfg.Source)
	}
	if cfg.Target.Owner != "community" || cfg.Target.Branch != "master" {
		t.Fatalf("legacy target was not derived from community: %#v", cfg.Target)
	}
	if cfg.Workflow.CommitRange != "community/master..private/queue" {
		t.Fatalf("commit range = %q", cfg.Workflow.CommitRange)
	}
}

func TestLoadDerivesBoundedCommitRangeFromStartAndEndSHA(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
local:
  path: "."
queue:
  remote: "private"
  branch: "new-features"
  base_ref: "community/master"
  start_sha: "47824259"
  end_sha: "1660a7c4"
private:
  remote: "private"
  branch_prefix: "mr-queue"
  head_namespace: "smileQiny"
community:
  remote: "community"
  owner: "openeuler"
  repo: "syskits"
  branch: "master"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
`)
	writeFile(t, envPath, `GITCODE_SUBMITTER_TOKEN=sub-token`)

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Workflow.CommitRange != "47824259^..1660a7c4" {
		t.Fatalf("commit range = %q", cfg.Workflow.CommitRange)
	}
}

func TestLoadMissingTokenReturnsClearError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")

	writeFile(t, configPath, `
source:
  local_path: "."
  base_ref: "main"
  remote: "origin"
  branch_prefix: "mr-queue"
  head_namespace: "submitter"
target:
  owner: "community"
  repo: "project"
  branch: "master"
workflow:
  commit_range: "main..HEAD"
auth:
  submitter:
    token_env: "MISSING_SUBMITTER_TOKEN"
`)

	err := os.Unsetenv("MISSING_SUBMITTER_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Load(configPath, "")
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if !strings.Contains(err.Error(), "MISSING_SUBMITTER_TOKEN") {
		t.Fatalf("error did not name missing env var: %v", err)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
