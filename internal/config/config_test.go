package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
  branch_template: "{prefix}-{title}-{sha12}"
  head_namespace: "submitter"
community:
  remote: "community"
  owner: "community"
  repo: "project"
  branch: "master"
workflow:
  merge_method: "squash"
  loop_delay_min: "30s"
  loop_delay_max: "90s"
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
	if cfg.Private.BranchTemplate != "{prefix}-{title}-{sha12}" {
		t.Fatalf("branch template = %q", cfg.Private.BranchTemplate)
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
	if !cfg.Workflow.ShouldApprove() {
		t.Fatal("approve should default to true")
	}
	if cfg.Workflow.LoopDelayMin != "30s" || cfg.Workflow.LoopDelayMax != "90s" {
		t.Fatalf("loop delay = %q..%q", cfg.Workflow.LoopDelayMin, cfg.Workflow.LoopDelayMax)
	}
}

func TestSafeRedactsRemoteURLCredentials(t *testing.T) {
	cfg := Config{
		Queue:     Queue{RemoteURL: "https://user:secret@gitcode.com/source/project.git"},
		Private:   Private{RemoteURL: "https://token@gitcode.com/source/project.git"},
		Community: Community{RemoteURL: "https://maintainer:secret@gitcode.com/target/project.git"},
	}

	safe := cfg.Safe()
	if strings.Contains(safe, "secret") || strings.Contains(safe, "token@") {
		t.Fatalf("Safe leaked remote URL credential: %s", safe)
	}
	if !strings.Contains(safe, "https://gitcode.com/source/project.git") {
		t.Fatalf("Safe removed queue remote URL host/path: %s", safe)
	}
}

func TestLoadSimpleModeDerivesLegacyQueuePrivateCommunity(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/Users/qiny/codespace/syskits"
source:
  repo: "gitcode.com/smileQiny/syskits"
  branch: "new-features"
  start_sha: "a3c47d5f"
  end_sha: "e34a0a61"
target:
  repo: "gitcode.com/openeuler/syskits"
  branch: "master"
mr:
  branch_prefix: "feat"
  branch_template: "{prefix}-{title_or_sha12}"
workflow:
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

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider != "gitcode" {
		t.Fatalf("provider = %q", cfg.Provider)
	}
	if cfg.Workspace != "/Users/qiny/codespace/syskits" || cfg.Local.Path != cfg.Workspace {
		t.Fatalf("workspace/local = %q/%q", cfg.Workspace, cfg.Local.Path)
	}
	if cfg.Source.Repo != "gitcode.com/smileQiny/syskits" {
		t.Fatalf("source repo = %q", cfg.Source.Repo)
	}
	if cfg.Target.Repo != "gitcode.com/openeuler/syskits" {
		t.Fatalf("target repo = %q", cfg.Target.Repo)
	}
	if cfg.Queue.Remote != "mrq-gitcode-com-smileqiny-syskits" {
		t.Fatalf("queue remote = %q", cfg.Queue.Remote)
	}
	if cfg.Queue.RemoteURL != "https://gitcode.com/smileQiny/syskits.git" {
		t.Fatalf("queue remote url = %q", cfg.Queue.RemoteURL)
	}
	if cfg.Queue.Branch != "new-features" {
		t.Fatalf("queue branch = %q", cfg.Queue.Branch)
	}
	if cfg.Queue.BaseRef != "mrq-gitcode-com-openeuler-syskits/master" {
		t.Fatalf("queue base ref = %q", cfg.Queue.BaseRef)
	}
	if cfg.Queue.StartSHA != "a3c47d5f" || cfg.Queue.EndSHA != "e34a0a61" {
		t.Fatalf("queue bounds = %q..%q", cfg.Queue.StartSHA, cfg.Queue.EndSHA)
	}
	if cfg.Workflow.CommitRange != "a3c47d5f^..e34a0a61" {
		t.Fatalf("commit range = %q", cfg.Workflow.CommitRange)
	}
	if cfg.Private.Remote != cfg.Queue.Remote || cfg.Private.RemoteURL != cfg.Queue.RemoteURL {
		t.Fatalf("private remote = %#v", cfg.Private)
	}
	if cfg.Private.HeadNamespace != "smileQiny" {
		t.Fatalf("head namespace = %q", cfg.Private.HeadNamespace)
	}
	if cfg.Private.BranchPrefix != "feat" || cfg.Private.BranchTemplate != "{prefix}-{title_or_sha12}" {
		t.Fatalf("private branch config = %#v", cfg.Private)
	}
	if cfg.Community.Remote != "mrq-gitcode-com-openeuler-syskits" {
		t.Fatalf("community remote = %q", cfg.Community.Remote)
	}
	if cfg.Community.RemoteURL != "https://gitcode.com/openeuler/syskits.git" {
		t.Fatalf("community remote url = %q", cfg.Community.RemoteURL)
	}
	if cfg.Community.Owner != "openeuler" || cfg.Community.Repo != "syskits" || cfg.Community.Branch != "master" {
		t.Fatalf("community = %#v", cfg.Community)
	}
}

func TestLoadSimpleModeUsesExplicitSourceRange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/repo"
source:
  repo: "gitcode.com/smileQiny/syskits"
  branch: "new-features"
  range: "abc123^..def456"
  start_sha: "ignored-start"
  end_sha: "ignored-end"
target:
  repo: "gitcode.com/openeuler/syskits"
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

	if cfg.Workflow.CommitRange != "abc123^..def456" {
		t.Fatalf("commit range = %q", cfg.Workflow.CommitRange)
	}
}

func TestLoadSimpleModeDefaultsCommitRangeToTargetBranchThroughSourceBranch(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/repo"
source:
  repo: "gitcode.com/smileQiny/syskits"
  branch: "new-features"
target:
  repo: "gitcode.com/openeuler/syskits"
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

	want := "mrq-gitcode-com-openeuler-syskits/master..mrq-gitcode-com-smileqiny-syskits/new-features"
	if cfg.Workflow.CommitRange != want {
		t.Fatalf("commit range = %q, want %q", cfg.Workflow.CommitRange, want)
	}
}

func TestLoadSimpleModeSupportsAtomGitRepositoryFullPaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: atomgit
workspace: "/Users/qiny/codespace/track-system"
source:
  repo: "atomgit.com/w-xxxx/track-system"
  branch: "w-xxxx/track-system-1.2.0"
target:
  repo: "atomgit.com/openeuler/track-system"
  branch: "master"
workflow:
  merge_method: "external"
auth:
  submitter:
    token_env: "ATOMGIT_SUBMITTER_TOKEN"
  reviewer:
    token_env: "ATOMGIT_REVIEWER_TOKEN"
`)
	writeFile(t, envPath, `
ATOMGIT_SUBMITTER_TOKEN=sub-token
ATOMGIT_REVIEWER_TOKEN=review-token
`)

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider != "atomgit" {
		t.Fatalf("provider = %q", cfg.Provider)
	}
	if cfg.Queue.Remote != "mrq-atomgit-com-w-xxxx-track-system" {
		t.Fatalf("queue remote = %q", cfg.Queue.Remote)
	}
	if cfg.Queue.RemoteURL != "https://atomgit.com/w-xxxx/track-system.git" {
		t.Fatalf("queue remote url = %q", cfg.Queue.RemoteURL)
	}
	if cfg.Community.Remote != "mrq-atomgit-com-openeuler-track-system" {
		t.Fatalf("community remote = %q", cfg.Community.Remote)
	}
	if cfg.Community.RemoteURL != "https://atomgit.com/openeuler/track-system.git" {
		t.Fatalf("community remote url = %q", cfg.Community.RemoteURL)
	}
	wantRange := "mrq-atomgit-com-openeuler-track-system/master..mrq-atomgit-com-w-xxxx-track-system/w-xxxx/track-system-1.2.0"
	if cfg.Workflow.CommitRange != wantRange {
		t.Fatalf("commit range = %q, want %q", cfg.Workflow.CommitRange, wantRange)
	}
}

func TestLoadSimpleModeRejectsUnsupportedRepositoryHost(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/repo"
source:
  repo: "gitee.com/smileQiny/syskits"
  branch: "new-features"
target:
  repo: "gitcode.com/openeuler/syskits"
  branch: "master"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
`)
	writeFile(t, envPath, `GITCODE_SUBMITTER_TOKEN=sub-token`)

	_, err := Load(configPath, envPath)
	if err == nil {
		t.Fatal("expected unsupported host error")
	}
	if !strings.Contains(err.Error(), "unsupported repository host gitee.com") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadSimpleModeRequiresRepositoryFullPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/repo"
source:
  repo: "smileQiny/syskits"
  branch: "new-features"
target:
  repo: "gitcode.com/openeuler/syskits"
  branch: "master"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
`)
	writeFile(t, envPath, `GITCODE_SUBMITTER_TOKEN=sub-token`)

	_, err := Load(configPath, envPath)
	if err == nil {
		t.Fatal("expected full repository path error")
	}
	if !strings.Contains(err.Error(), "source.repo must include repository host") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadSimpleModeRequiresSourceBranchOrRange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
provider: gitcode
workspace: "/repo"
source:
  repo: "gitcode.com/smileQiny/syskits"
target:
  repo: "gitcode.com/openeuler/syskits"
  branch: "master"
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
`)
	writeFile(t, envPath, `GITCODE_SUBMITTER_TOKEN=sub-token`)

	_, err := Load(configPath, envPath)
	if err == nil {
		t.Fatal("expected missing source branch error")
	}
	if !strings.Contains(err.Error(), "source.branch is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestWorkflowDefaultsSplitWaitAndNextPRDelayRanges(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()

	if cfg.Workflow.WaitCheckDelayMin != "10s" || cfg.Workflow.WaitCheckDelayMax != "30s" {
		t.Fatalf("wait check delay defaults = %q..%q", cfg.Workflow.WaitCheckDelayMin, cfg.Workflow.WaitCheckDelayMax)
	}
	if cfg.Workflow.NextPRDelayMin != "1m" || cfg.Workflow.NextPRDelayMax != "5m" {
		t.Fatalf("next PR delay defaults = %q..%q", cfg.Workflow.NextPRDelayMin, cfg.Workflow.NextPRDelayMax)
	}
	minDelay, maxDelay, err := cfg.Workflow.WaitCheckDelayRange()
	if err != nil {
		t.Fatalf("WaitCheckDelayRange returned error: %v", err)
	}
	if minDelay != 10*time.Second || maxDelay != 30*time.Second {
		t.Fatalf("parsed wait check delay defaults = %s..%s", minDelay, maxDelay)
	}
	minDelay, maxDelay, err = cfg.Workflow.NextPRDelayRange()
	if err != nil {
		t.Fatalf("NextPRDelayRange returned error: %v", err)
	}
	if minDelay != time.Minute || maxDelay != 5*time.Minute {
		t.Fatalf("parsed next PR delay defaults = %s..%s", minDelay, maxDelay)
	}
}

func TestLoadReadsApproveFalse(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mr-queue.yml")
	envPath := filepath.Join(dir, ".env")

	writeFile(t, configPath, `
local:
  path: "."
workflow:
  commit_range: "main..HEAD"
  approve: false
auth:
  submitter:
    token_env: "GITCODE_SUBMITTER_TOKEN"
`)
	writeFile(t, envPath, `GITCODE_SUBMITTER_TOKEN=sub-token`)

	cfg, err := Load(configPath, envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Workflow.ShouldApprove() {
		t.Fatal("approve should be false")
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
