package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalGitCheckerDetectsAndFixesRemote(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0700); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "init", "--bare")
	runGit(t, dir, "init")

	checker := LocalGitChecker{Dir: dir}
	state, err := checker.RemoteState("mrq-source", remote)
	if err != nil {
		t.Fatalf("RemoteState returned error: %v", err)
	}
	if state.Exists {
		t.Fatalf("remote unexpectedly exists: %#v", state)
	}

	if err := checker.EnsureRemote("mrq-source", remote); err != nil {
		t.Fatalf("EnsureRemote returned error: %v", err)
	}
	state, err = checker.RemoteState("mrq-source", remote)
	if err != nil {
		t.Fatalf("RemoteState returned error: %v", err)
	}
	if !state.Exists || state.URL != remote {
		t.Fatalf("remote state = %#v", state)
	}
}

func TestLocalGitCheckerChecksCommitRange(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(dir, "base.txt"), "base\n")
	runGit(t, dir, "add", "base.txt")
	runGit(t, dir, "commit", "-m", "base")
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "feature.txt"), "feature\n")
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-m", "feature")

	checker := LocalGitChecker{Dir: dir}
	if err := checker.CheckCommitRange("master..feature"); err != nil {
		t.Fatalf("CheckCommitRange returned error: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
