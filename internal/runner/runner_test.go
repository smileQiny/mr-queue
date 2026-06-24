package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mr-queue/internal/config"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/state"
)

type fakeGitOps struct {
	refreshed []string
	pushed    []string
}

func (g *fakeGitOps) RefreshRefs(remotes []string) error {
	g.refreshed = append(g.refreshed, remotes...)
	return nil
}

func (g *fakeGitOps) ListCommits(commitRange string) ([]Commit, error) {
	return []Commit{{
		SHA:     "abcdef123456",
		Subject: "Add feature",
		Body:    "Body text",
	}}, nil
}

func (g *fakeGitOps) PushSingleCommitBranch(commit Commit, branchName string, baseRef string, remote string) (string, error) {
	g.pushed = append(g.pushed, commit.SHA+":"+branchName+":"+baseRef+":"+remote)
	return "mr-" + commit.SHA, nil
}

type fakeClient struct {
	actions []string
}

func (c *fakeClient) CreatePull(owner string, repo string, input gitcode.PullRequestInput) (gitcode.PullRequest, error) {
	c.actions = append(c.actions, "create:"+input.Head)
	return gitcode.PullRequest{Number: 9, HTMLURL: "https://gitcode.com/community/project/pulls/9"}, nil
}

func (c *fakeClient) CommentPull(owner string, repo string, number int, body string) (gitcode.Comment, error) {
	c.actions = append(c.actions, "comment")
	return gitcode.Comment{ID: 1}, nil
}

func (c *fakeClient) ReviewPull(owner string, repo string, number int) (gitcode.Review, error) {
	c.actions = append(c.actions, "review")
	return gitcode.Review{}, nil
}

func (c *fakeClient) MergePull(owner string, repo string, number int, input gitcode.MergeInput) (gitcode.MergeResult, error) {
	c.actions = append(c.actions, "merge:"+input.MergeMethod)
	return gitcode.MergeResult{Merged: true, SHA: "community-merge-sha"}, nil
}

func TestRunOnceProcessesOneCommitToMergedState(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	gitOps := &fakeGitOps{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			Branch:  "queue",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:        "private",
			BranchPrefix:  "mr-queue",
			HeadNamespace: "submitter",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "community",
			Repo:   "project",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "squash",
			ReviewComment: "Reviewed and approved.",
		},
	}, store, gitOps, submitter, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusMerged {
		t.Fatalf("status = %q", task.Status)
	}
	if task.MRNumber != 9 {
		t.Fatalf("mr number = %d", task.MRNumber)
	}
	if task.Branch != "mr-queue-abcdef123456" {
		t.Fatalf("branch = %q", task.Branch)
	}
	if task.MRCommitSHA != "mr-abcdef123456" {
		t.Fatalf("mr commit sha = %q", task.MRCommitSHA)
	}
	if task.CommunityCommitSHA != "community-merge-sha" {
		t.Fatalf("community commit sha = %q", task.CommunityCommitSHA)
	}
	if gitOps.pushed[0] != "abcdef123456:mr-queue-abcdef123456:community/master:private" {
		t.Fatalf("pushed = %#v", gitOps.pushed)
	}
	if reviewer.actions[0] != "comment" || reviewer.actions[1] != "review" {
		t.Fatalf("reviewer actions = %#v", reviewer.actions)
	}
	if maintainer.actions[0] != "merge:squash" {
		t.Fatalf("maintainer actions = %#v", maintainer.actions)
	}
}

func TestRunOnceDoesNotRestartInFlightTask(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTask("abcdef123456", "Add feature"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abcdef123456", state.StatusMROpen, ""); err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	gitOps := &fakeGitOps{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			Branch:  "new-features",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:        "private",
			BranchPrefix:  "mr-queue",
			HeadNamespace: "smileQiny",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "47824259^..1660a7c4",
			MergeMethod:   "squash",
			ReviewComment: "Reviewed and approved.",
		},
	}, store, gitOps, submitter, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(gitOps.pushed) != 0 {
		t.Fatalf("unexpected push for in-flight task: %#v", gitOps.pushed)
	}
	if len(submitter.actions) != 0 || len(reviewer.actions) != 0 || len(maintainer.actions) != 0 {
		t.Fatalf("unexpected API actions: submitter=%#v reviewer=%#v maintainer=%#v", submitter.actions, reviewer.actions, maintainer.actions)
	}
}

func TestLocalGitOpsListCommitsReturnsEmptyForRepoWithoutCommits(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")

	commits, err := LocalGitOps{Dir: dir}.ListCommits("main..HEAD")
	if err != nil {
		t.Fatalf("ListCommits returned error: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("commits = %#v", commits)
	}
}

func TestLocalGitOpsPushSingleCommitBranchCreatesBranchWithOnlyOneCommitOverBase(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0700); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "init", "--bare")
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, dir, "remote", "add", "origin", remote)

	writeRepoFile(t, dir, "base.txt", "base\n")
	runGit(t, dir, "add", "base.txt")
	runGit(t, dir, "commit", "-m", "base")
	base := gitOutput(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "push", "origin", "main")

	writeRepoFile(t, dir, "one.txt", "one\n")
	runGit(t, dir, "add", "one.txt")
	runGit(t, dir, "commit", "-m", "one")
	first := gitOutput(t, dir, "rev-parse", "HEAD")

	writeRepoFile(t, dir, "two.txt", "two\n")
	runGit(t, dir, "add", "two.txt")
	runGit(t, dir, "commit", "-m", "two")
	second := gitOutput(t, dir, "rev-parse", "HEAD")

	mrCommitSHA, err := LocalGitOps{Dir: dir}.PushSingleCommitBranch(Commit{SHA: second}, "mr-queue-second", base, "origin")
	if err != nil {
		t.Fatalf("PushSingleCommitBranch returned error: %v", err)
	}

	branchTip := gitOutput(t, dir, "rev-parse", "origin/mr-queue-second")
	count := gitOutput(t, dir, "rev-list", "--count", base+".."+branchTip)
	if count != "1" {
		t.Fatalf("commit count over base = %s", count)
	}
	subject := gitOutput(t, dir, "log", "-1", "--format=%s", branchTip)
	if subject != "two" {
		t.Fatalf("branch subject = %q", subject)
	}
	if mrCommitSHA != branchTip {
		t.Fatalf("mr commit sha = %q, want %q", mrCommitSHA, branchTip)
	}
	if strings.TrimSpace(first) == strings.TrimSpace(branchTip) {
		t.Fatalf("branch unexpectedly points at first commit")
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

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func writeRepoFile(t *testing.T, dir string, name string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
