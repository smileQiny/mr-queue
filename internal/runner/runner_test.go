package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mr-queue/internal/config"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/state"
)

type fakeGitOps struct {
	refreshed []string
	pushed    []string
	commits   []Commit
	pushErr   error
}

func (g *fakeGitOps) RefreshRefs(remotes []string) error {
	g.refreshed = append(g.refreshed, remotes...)
	return nil
}

func (g *fakeGitOps) ListCommits(commitRange string) ([]Commit, error) {
	if g.commits != nil {
		return g.commits, nil
	}
	return []Commit{{
		SHA:     "abcdef123456",
		Subject: "Add feature",
		Body:    "Body text",
	}}, nil
}

func (g *fakeGitOps) PushSingleCommitBranch(commit Commit, branchName string, baseRef string, remote string) (string, error) {
	g.pushed = append(g.pushed, commit.SHA+":"+branchName+":"+baseRef+":"+remote)
	if g.pushErr != nil {
		return "", g.pushErr
	}
	return "mr-" + commit.SHA, nil
}

type fakeClient struct {
	actions       []string
	commentBodies []string
	comments      []gitcode.Comment
	pull          gitcode.PullRequest
	pulls         []gitcode.PullRequest
	reviewErr     error
	nextNumber    int
}

func (c *fakeClient) CreatePull(owner string, repo string, input gitcode.PullRequestInput) (gitcode.PullRequest, error) {
	c.actions = append(c.actions, "create:"+input.Head)
	number := c.nextNumber
	if number == 0 {
		number = 9
	}
	c.nextNumber = number + 1
	return gitcode.PullRequest{Number: number, HTMLURL: "https://gitcode.com/community/project/pulls/9"}, nil
}

func (c *fakeClient) GetPull(owner string, repo string, number int) (gitcode.PullRequest, error) {
	c.actions = append(c.actions, "get-pull")
	if len(c.pulls) > 0 {
		pull := c.pulls[0]
		c.pulls = c.pulls[1:]
		return pull, nil
	}
	if c.pull.Number == 0 {
		return gitcode.PullRequest{Number: number}, nil
	}
	return c.pull, nil
}

func (c *fakeClient) CommentPull(owner string, repo string, number int, body string) (gitcode.Comment, error) {
	c.actions = append(c.actions, "comment")
	c.commentBodies = append(c.commentBodies, body)
	return gitcode.Comment{ID: "1"}, nil
}

func (c *fakeClient) ListPullComments(owner string, repo string, number int) ([]gitcode.Comment, error) {
	c.actions = append(c.actions, "list-comments")
	return c.comments, nil
}

func (c *fakeClient) ReviewPull(owner string, repo string, number int) (gitcode.Review, error) {
	c.actions = append(c.actions, "review")
	if c.reviewErr != nil {
		return gitcode.Review{}, c.reviewErr
	}
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
	if task.QueueIndex != 0 {
		t.Fatalf("queue index = %d", task.QueueIndex)
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

func TestRunOnceUsesPlainHeadBranchForSameRepositoryMR(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:        "private",
			BranchPrefix:  "new-features-commit",
			HeadNamespace: "smileQiny",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange: "main..HEAD",
			MergeMethod: "squash",
		},
	}, store, &fakeGitOps{}, submitter, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if submitter.actions[0] != "create:new-features-commit-abcdef123456" {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestRunOnceUsesConfiguredBranchTemplate(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	gitOps := &fakeGitOps{commits: []Commit{{
		SHA:     "abcdef1234567890",
		Subject: "[stat] 清理静态检查问题",
	}}}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:         "private",
			BranchPrefix:   "feat",
			BranchTemplate: "{prefix}-{title}-{sha12}",
			HeadNamespace:  "smileQiny",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange: "main..HEAD",
			MergeMethod: "squash",
		},
	}, store, gitOps, submitter, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef1234567890"]
	if task.Branch != "feat-stat-abcdef123456" {
		t.Fatalf("branch = %q", task.Branch)
	}
	if gitOps.pushed[0] != "abcdef1234567890:feat-stat-abcdef123456:private/master-test:private" {
		t.Fatalf("pushed = %#v", gitOps.pushed)
	}
	if submitter.actions[0] != "create:feat-stat-abcdef123456" {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestBranchTemplateTitleOrSHA12UsesTitleWhenAvailable(t *testing.T) {
	got := branchName("{prefix}-{title_or_sha12}", "feat", Commit{
		SHA:     "abcdef1234567890",
		Subject: "[stat] Add localized help resources",
	})
	if got != "feat-stat-add-localized-help-resources" {
		t.Fatalf("branch = %q", got)
	}
}

func TestBranchTemplateTitleOrSHA12FallsBackToSHA12(t *testing.T) {
	got := branchName("{prefix}-{title_or_sha12}", "feat", Commit{
		SHA:     "abcdef1234567890",
		Subject: "清理静态检查问题",
	})
	if got != "feat-abcdef123456" {
		t.Fatalf("branch = %q", got)
	}
}

func TestRunOnceUsesNamespacedHeadBranchForCrossRepositoryMR(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:        "private",
			BranchPrefix:  "new-features-commit",
			HeadNamespace: "smileQiny",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange: "main..HEAD",
			MergeMethod: "squash",
		},
	}, store, &fakeGitOps{}, submitter, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if submitter.actions[0] != "create:smileQiny:new-features-commit-abcdef123456" {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestRunOnceResumesExistingMROnRetry(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("abcdef123456", "Add feature", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskBranch("abcdef123456", "mr-queue-abcdef123456"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMRCommit("abcdef123456", "mr-abcdef123456"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("abcdef123456", 9, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.RetryTask("abcdef123456"); err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	gitOps := &fakeGitOps{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
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

	if len(gitOps.pushed) != 0 {
		t.Fatalf("unexpected push: %#v", gitOps.pushed)
	}
	if len(submitter.actions) != 0 {
		t.Fatalf("unexpected create MR: %#v", submitter.actions)
	}
	if reviewer.actions[0] != "comment" || reviewer.actions[1] != "review" {
		t.Fatalf("reviewer actions = %#v", reviewer.actions)
	}
	if maintainer.actions[0] != "merge:squash" {
		t.Fatalf("maintainer actions = %#v", maintainer.actions)
	}
	if store.Snapshot().Tasks["abcdef123456"].Status != state.StatusMerged {
		t.Fatalf("status = %q", store.Snapshot().Tasks["abcdef123456"].Status)
	}
}

func TestRunOnceCanSkipApprovalWhenConfigured(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "squash",
			ReviewComment: "Reviewed and approved.",
			Approve:       boolPtr(false),
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(reviewer.actions) != 1 || reviewer.actions[0] != "comment" {
		t.Fatalf("reviewer actions = %#v", reviewer.actions)
	}
	if maintainer.actions[0] != "merge:squash" {
		t.Fatalf("maintainer actions = %#v", maintainer.actions)
	}
}

func TestRunOnceWaitsForRequiredCommentBeforeReviewCommand(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "squash",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusWaitingRequiredComment {
		t.Fatalf("status = %q", task.Status)
	}
	if len(reviewer.commentBodies) != 0 {
		t.Fatalf("unexpected reviewer comments: %#v", reviewer.commentBodies)
	}
	if len(maintainer.actions) != 0 {
		t.Fatalf("unexpected merge actions: %#v", maintainer.actions)
	}

	reviewer.comments = []gitcode.Comment{{Body: "CLA Signature Pass"}}
	if err := r.RunOnce(); err != nil {
		t.Fatalf("second RunOnce returned error: %v", err)
	}

	task = store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusMerged {
		t.Fatalf("status after CLA = %q", task.Status)
	}
	if len(reviewer.commentBodies) != 1 || reviewer.commentBodies[0] != "/lgtm\n/approve" {
		t.Fatalf("review comments = %#v", reviewer.commentBodies)
	}
	if maintainer.actions[0] != "merge:squash" {
		t.Fatalf("maintainer actions = %#v", maintainer.actions)
	}
}

func TestRunOnceExternalMergeStopsAfterReviewCommand(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{comments: []gitcode.Comment{{Body: "CLA Signature Pass"}}}
	maintainer := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "external",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusWaitingExternalMerge {
		t.Fatalf("status = %q", task.Status)
	}
	if len(reviewer.commentBodies) != 1 || reviewer.commentBodies[0] != "/lgtm\n/approve" {
		t.Fatalf("review comments = %#v", reviewer.commentBodies)
	}
	if len(maintainer.actions) != 0 {
		t.Fatalf("unexpected maintainer actions: %#v", maintainer.actions)
	}
}

func TestRunOnceExternalMergeMarksMergedWhenPullIsMerged(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("abcdef123456", "Add feature", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("abcdef123456", 9, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abcdef123456", state.StatusWaitingExternalMerge, ""); err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{pull: gitcode.PullRequest{
		Number:         9,
		State:          "merged",
		Merged:         true,
		MergeCommitSHA: "merged-by-bot",
	}}
	maintainer := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "external",
			ReviewComment: "/lgtm\n/approve",
			Approve:       boolPtr(false),
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusMerged {
		t.Fatalf("status = %q", task.Status)
	}
	if task.CommunityCommitSHA != "merged-by-bot" {
		t.Fatalf("community commit = %q", task.CommunityCommitSHA)
	}
	if len(maintainer.actions) != 0 {
		t.Fatalf("unexpected maintainer actions: %#v", maintainer.actions)
	}
}

func TestRunLoopKeepsPollingWaitingExternalMerge(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("abcdef123456", "Add feature", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("abcdef123456", 9, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abcdef123456", state.StatusWaitingExternalMerge, ""); err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{pull: gitcode.PullRequest{Number: 9, State: "open"}}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "external",
			ReviewComment: "/lgtm\n/approve",
			Approve:       boolPtr(false),
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, &fakeClient{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err = r.RunLoopRange(ctx, time.Millisecond, time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunLoopRange returned %v, want context deadline", err)
	}
	if len(reviewer.actions) < 2 {
		t.Fatalf("expected repeated polling, actions = %#v", reviewer.actions)
	}
}

func TestRunLoopStopsAfterConfiguredMergedCount(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("first123456789", "First", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("second12345678", "Second", 1); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "first123456789", Subject: "First"},
			{SHA: "second12345678", Subject: "Second"},
		},
	}
	submitter := &fakeClient{nextNumber: 11}
	reviewer := &fakeClient{
		comments: []gitcode.Comment{{Body: "CLA Signature Pass"}},
		pulls: []gitcode.PullRequest{
			{Number: 11, State: "merged", Merged: true, MergeCommitSHA: "first-merged"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "external",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, gitOps, submitter, reviewer, &fakeClient{})

	result, err := r.RunLoopWithOptions(context.Background(), LoopOptions{
		MinDelay:         time.Millisecond,
		MaxDelay:         time.Millisecond,
		MaxMergedCommits: 1,
	})
	if err != nil {
		t.Fatalf("RunLoopWithOptions returned error: %v", err)
	}
	if result.StopReason != "reached max merged commits: 1/1" {
		t.Fatalf("stop reason = %q", result.StopReason)
	}

	tasks := store.Snapshot().Tasks
	if tasks["first123456789"].Status != state.StatusMerged {
		t.Fatalf("first status = %q", tasks["first123456789"].Status)
	}
	if tasks["second12345678"].Status != state.StatusPending {
		t.Fatalf("second status = %q", tasks["second12345678"].Status)
	}
	if len(submitter.actions) != 1 {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestRunLoopMergedLimitIgnoresTasksAlreadyWaitingAtStart(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("alreadywaiting1", "Already waiting", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("alreadywaiting1", 10, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("alreadywaiting1", state.StatusWaitingExternalMerge, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("firstpending12", "First pending", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("secondpending1", "Second pending", 2); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "alreadywaiting1", Subject: "Already waiting"},
			{SHA: "firstpending12", Subject: "First pending"},
			{SHA: "secondpending1", Subject: "Second pending"},
		},
	}
	submitter := &fakeClient{nextNumber: 11}
	reviewer := &fakeClient{
		comments: []gitcode.Comment{{Body: "CLA Signature Pass"}},
		pulls: []gitcode.PullRequest{
			{Number: 10, State: "merged", Merged: true, MergeCommitSHA: "already-merged"},
			{Number: 11, State: "merged", Merged: true, MergeCommitSHA: "first-merged"},
			{Number: 12, State: "merged", Merged: true, MergeCommitSHA: "second-merged"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "external",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, gitOps, submitter, reviewer, &fakeClient{})

	_, err = r.RunLoopWithOptions(context.Background(), LoopOptions{
		WaitCheckDelayMin: time.Millisecond,
		WaitCheckDelayMax: time.Millisecond,
		NextPRDelayMin:    time.Millisecond,
		NextPRDelayMax:    time.Millisecond,
		MaxMergedCommits:  2,
	})
	if err != nil {
		t.Fatalf("RunLoopWithOptions returned error: %v", err)
	}

	tasks := store.Snapshot().Tasks
	if tasks["alreadywaiting1"].Status != state.StatusMerged {
		t.Fatalf("already waiting status = %q", tasks["alreadywaiting1"].Status)
	}
	if tasks["firstpending12"].Status != state.StatusMerged {
		t.Fatalf("first pending status = %q", tasks["firstpending12"].Status)
	}
	if tasks["secondpending1"].Status != state.StatusMerged {
		t.Fatalf("second pending status = %q", tasks["secondpending1"].Status)
	}
	if len(submitter.actions) != 2 {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestRunLoopStopsOutsideConfiguredWorkWindow(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{{SHA: "first123456789", Subject: "First"}},
	}
	submitter := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "squash",
			ReviewComment: "Reviewed.",
		},
	}, store, gitOps, submitter, &fakeClient{}, &fakeClient{})

	_, err = r.RunLoopWithOptions(context.Background(), LoopOptions{
		MinDelay:        time.Millisecond,
		MaxDelay:        time.Millisecond,
		WorkWindowStart: "08:00",
		WorkWindowEnd:   "23:30",
		Now: func() time.Time {
			return time.Date(2026, 6, 24, 7, 59, 0, 0, time.Local)
		},
	})
	if err != nil {
		t.Fatalf("RunLoopWithOptions returned error: %v", err)
	}
	if len(gitOps.pushed) != 0 {
		t.Fatalf("unexpected push outside work window: %#v", gitOps.pushed)
	}
	if len(submitter.actions) != 0 {
		t.Fatalf("unexpected MR outside work window: %#v", submitter.actions)
	}
}

func TestRunLoopUsesWaitCheckDelayUntilMergedThenNextPRDelay(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "first123456789", Subject: "First"},
			{SHA: "second12345678", Subject: "Second"},
		},
	}
	submitter := &fakeClient{nextNumber: 11}
	reviewer := &fakeClient{
		comments: []gitcode.Comment{{Body: "CLA Signature Pass"}},
		pulls: []gitcode.PullRequest{
			{Number: 11, State: "open"},
			{Number: 11, State: "merged", Merged: true, MergeCommitSHA: "first-merged"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "external",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, gitOps, submitter, reviewer, &fakeClient{})

	var sleeps []time.Duration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = r.RunLoopWithOptions(ctx, LoopOptions{
		WaitCheckDelayMin: time.Second,
		WaitCheckDelayMax: time.Second,
		NextPRDelayMin:    5 * time.Second,
		NextPRDelayMax:    5 * time.Second,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			sleeps = append(sleeps, delay)
			if len(sleeps) >= 3 {
				cancel()
				return context.Canceled
			}
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunLoopWithOptions returned %v, want context canceled", err)
	}

	if len(sleeps) != 3 {
		t.Fatalf("sleeps = %#v", sleeps)
	}
	if sleeps[0] != time.Second || sleeps[1] != time.Second {
		t.Fatalf("wait sleeps = %#v", sleeps[:2])
	}
	if sleeps[2] != 5*time.Second {
		t.Fatalf("next PR sleep = %s", sleeps[2])
	}
	if len(submitter.actions) != 1 {
		t.Fatalf("submitter actions = %#v", submitter.actions)
	}
}

func TestRunLoopReportsNextPRDelayProgress(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "first123456789", Subject: "First"},
			{SHA: "second12345678", Subject: "Second"},
		},
	}
	submitter := &fakeClient{nextNumber: 11}
	reviewer := &fakeClient{
		comments: []gitcode.Comment{{Body: "CLA Signature Pass"}},
		pulls: []gitcode.PullRequest{
			{Number: 11, State: "merged", Merged: true, MergeCommitSHA: "first-merged"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "external",
			ReviewComment:       "/lgtm\n/approve",
			RequiredCommentText: "CLA Signature Pass",
			Approve:             boolPtr(false),
		},
	}, store, gitOps, submitter, reviewer, &fakeClient{})

	var progress []LoopProgress
	progressSeen := false
	sleepCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = r.RunLoopWithOptions(ctx, LoopOptions{
		WaitCheckDelayMin: time.Millisecond,
		WaitCheckDelayMax: time.Millisecond,
		NextPRDelayMin:    2 * time.Minute,
		NextPRDelayMax:    2 * time.Minute,
		MaxMergedCommits:  3,
		OnProgress: func(item LoopProgress) {
			progress = append(progress, item)
			progressSeen = true
			cancel()
		},
		Sleep: func(ctx context.Context, delay time.Duration) error {
			sleepCount++
			if progressSeen {
				return context.Canceled
			}
			if sleepCount > 5 {
				return context.Canceled
			}
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunLoopWithOptions returned %v, want context canceled", err)
	}
	if len(progress) != 1 {
		t.Fatalf("progress = %#v", progress)
	}
	if progress[0].Delay != 2*time.Minute {
		t.Fatalf("delay = %s", progress[0].Delay)
	}
	if progress[0].MergedCount != 1 || progress[0].TargetMerged != 3 {
		t.Fatalf("progress counts = %d/%d", progress[0].MergedCount, progress[0].TargetMerged)
	}
	if progress[0].Message != "本轮计划合入 3 个 PR，当前已合入 1 个；下个 PR 将在 2m0s 后启动" {
		t.Fatalf("message = %q", progress[0].Message)
	}
}

func TestRefreshWaitingDoesNotCreateNewPulls(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("merged123456", "Merged", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("merged123456", state.StatusMerged, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("waiting12345", "Waiting", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("waiting12345", 9, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("waiting12345", state.StatusWaitingExternalMerge, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("pending12345", "Pending", 2); err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{pull: gitcode.PullRequest{
		Number:         9,
		State:          "merged",
		MergeCommitSHA: "merged-by-bot",
	}}
	submitter := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "main..HEAD",
			MergeMethod:   "external",
			ReviewComment: "/lgtm\n/approve",
			Approve:       boolPtr(false),
		},
	}, store, &fakeGitOps{}, submitter, reviewer, &fakeClient{})

	if err := r.RefreshWaiting(); err != nil {
		t.Fatalf("RefreshWaiting returned error: %v", err)
	}

	tasks := store.Snapshot().Tasks
	if tasks["waiting12345"].Status != state.StatusMerged {
		t.Fatalf("waiting status = %q", tasks["waiting12345"].Status)
	}
	if tasks["pending12345"].Status != state.StatusPending {
		t.Fatalf("pending status = %q", tasks["pending12345"].Status)
	}
	if len(submitter.actions) != 0 {
		t.Fatalf("unexpected submitter actions: %#v", submitter.actions)
	}
}

func TestRunOnceCanWarnAndContinueOnApprovalFailure(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reviewer := &fakeClient{reviewErr: errors.New("403 approval forbidden")}
	maintainer := &fakeClient{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange:         "main..HEAD",
			MergeMethod:         "squash",
			ReviewComment:       "Reviewed and approved.",
			ApprovalFailureMode: "warn",
		},
	}, store, &fakeGitOps{}, &fakeClient{}, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if reviewer.actions[0] != "comment" || reviewer.actions[1] != "review" {
		t.Fatalf("reviewer actions = %#v", reviewer.actions)
	}
	if maintainer.actions[0] != "merge:squash" {
		t.Fatalf("maintainer actions = %#v", maintainer.actions)
	}
	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusMerged {
		t.Fatalf("status = %q", task.Status)
	}
	foundWarning := false
	for _, log := range task.Logs {
		if log.Step == "approval" {
			foundWarning = true
			if strings.Contains(log.Message, "403 approval forbidden") {
				t.Fatalf("approval warning leaked raw error: %q", log.Message)
			}
		}
	}
	if !foundWarning {
		t.Fatalf("missing approval warning log: %#v", task.Logs)
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

func TestRunOnceDoesNotRestartSkippedTask(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTask("abcdef123456", "Add feature"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abcdef123456", state.StatusSkipped, ""); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange: "47824259^..1660a7c4",
		},
	}, store, gitOps, &fakeClient{}, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if len(gitOps.pushed) != 0 {
		t.Fatalf("unexpected push for skipped task: %#v", gitOps.pushed)
	}
}

func TestRunOnceStoresQueueIndexForCommitItProcesses(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("first12345678", "First feature", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("first12345678", state.StatusMerged, ""); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "first12345678", Subject: "First feature"},
			{SHA: "second1234567", Subject: "Second feature"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "community",
			Repo:   "project",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange: "main..HEAD",
			MergeMethod: "squash",
		},
	}, store, gitOps, &fakeClient{}, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["second1234567"]
	if task.Status != state.StatusMerged {
		t.Fatalf("status = %q", task.Status)
	}
	if task.QueueIndex != 1 {
		t.Fatalf("queue index = %d", task.QueueIndex)
	}
}

func TestRunOnceSkipsEmptyCherryPickWithoutCreatingMR(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	gitOps := &fakeGitOps{
		pushErr: ErrEmptyCherryPick,
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "private/master-test",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "private",
			Owner:  "smileQiny",
			Repo:   "syskits",
			Branch: "master-test",
		},
		Workflow: config.Workflow{
			CommitRange: "47824259^..1660a7c4",
			MergeMethod: "squash",
		},
	}, store, gitOps, submitter, reviewer, maintainer)

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	task := store.Snapshot().Tasks["abcdef123456"]
	if task.Status != state.StatusSkipped {
		t.Fatalf("status = %q", task.Status)
	}
	if task.Error != "" {
		t.Fatalf("error = %q", task.Error)
	}
	if task.MRNumber != 0 {
		t.Fatalf("mr number = %d", task.MRNumber)
	}
	if !errors.Is(gitOps.pushErr, ErrEmptyCherryPick) {
		t.Fatalf("push err = %v", gitOps.pushErr)
	}
	if len(submitter.actions) != 0 || len(reviewer.actions) != 0 || len(maintainer.actions) != 0 {
		t.Fatalf("unexpected API actions: submitter=%#v reviewer=%#v maintainer=%#v", submitter.actions, reviewer.actions, maintainer.actions)
	}
}

func TestRunOnceReplacesPreviousQueueRange(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("oldabcdef123", "Old waiting task", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMR("oldabcdef123", 99, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("oldabcdef123", state.StatusWaitingExternalMerge, ""); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "newabcdef123", Subject: "New range task"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote:  "private",
			BaseRef: "community/master",
		},
		Private: config.Private{
			Remote:       "private",
			BranchPrefix: "mr-queue",
		},
		Community: config.Community{
			Remote: "community",
			Owner:  "openeuler",
			Repo:   "syskits",
			Branch: "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "new-start^..new-end",
			MergeMethod:   "squash",
			ReviewComment: "Reviewed.",
		},
	}, store, gitOps, &fakeClient{}, &fakeClient{}, &fakeClient{})

	if err := r.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	tasks := store.Snapshot().Tasks
	if _, ok := tasks["oldabcdef123"]; ok {
		t.Fatalf("old range task still exists: %#v", tasks["oldabcdef123"])
	}
	if len(tasks) != 1 {
		t.Fatalf("task count = %d", len(tasks))
	}
	if tasks["newabcdef123"].QueueIndex != 0 {
		t.Fatalf("new queue index = %d", tasks["newabcdef123"].QueueIndex)
	}
}

func TestSyncQueueLoadsPendingTasksWithoutCreatingMRs(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	submitter := &fakeClient{}
	reviewer := &fakeClient{}
	maintainer := &fakeClient{}
	gitOps := &fakeGitOps{}
	gitOps.commits = []Commit{
		{SHA: "abcdef123456", Subject: "Add feature"},
		{SHA: "123456abcdef", Subject: "Fix bug"},
	}
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
			CommitRange: "47824259^..1660a7c4",
		},
	}, store, gitOps, submitter, reviewer, maintainer)

	count, err := r.SyncQueue()
	if err != nil {
		t.Fatalf("SyncQueue returned error: %v", err)
	}

	if count != 2 {
		t.Fatalf("count = %d", count)
	}
	tasks := store.Snapshot().Tasks
	task := tasks["abcdef123456"]
	if task.Status != state.StatusPending {
		t.Fatalf("status = %q", task.Status)
	}
	if task.QueueIndex != 0 {
		t.Fatalf("queue index = %d", task.QueueIndex)
	}
	second := tasks["123456abcdef"]
	if second.QueueIndex != 1 {
		t.Fatalf("second queue index = %d", second.QueueIndex)
	}
	if len(gitOps.pushed) != 0 {
		t.Fatalf("unexpected pushes: %#v", gitOps.pushed)
	}
	if len(submitter.actions) != 0 || len(reviewer.actions) != 0 || len(maintainer.actions) != 0 {
		t.Fatalf("unexpected API actions: submitter=%#v reviewer=%#v maintainer=%#v", submitter.actions, reviewer.actions, maintainer.actions)
	}
}

func TestSyncQueueReplacesPreviousQueueRange(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("oldabcdef123", "Old range task", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("oldabcdef123", state.StatusFailed, "old failure"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("keepabcdef12", "Keep old subject", 9); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("keepabcdef12", state.StatusMerged, ""); err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{
			{SHA: "keepabcdef12", Subject: "Keep new subject"},
			{SHA: "newabcdef123", Subject: "New range task"},
		},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote: "private",
		},
		Community: config.Community{
			Remote: "private",
		},
		Workflow: config.Workflow{
			CommitRange: "new-start^..new-end",
		},
	}, store, gitOps, &fakeClient{}, &fakeClient{}, &fakeClient{})

	count, err := r.SyncQueue()
	if err != nil {
		t.Fatalf("SyncQueue returned error: %v", err)
	}

	if count != 2 {
		t.Fatalf("count = %d", count)
	}
	tasks := store.Snapshot().Tasks
	if _, ok := tasks["oldabcdef123"]; ok {
		t.Fatalf("old range task still exists: %#v", tasks["oldabcdef123"])
	}
	if len(tasks) != 2 {
		t.Fatalf("task count = %d", len(tasks))
	}
	if tasks["keepabcdef12"].Status != state.StatusMerged {
		t.Fatalf("keep status = %q", tasks["keepabcdef12"].Status)
	}
	if tasks["keepabcdef12"].QueueIndex != 0 {
		t.Fatalf("keep queue index = %d", tasks["keepabcdef12"].QueueIndex)
	}
	if tasks["newabcdef123"].QueueIndex != 1 {
		t.Fatalf("new queue index = %d", tasks["newabcdef123"].QueueIndex)
	}
}

func TestSyncQueueCanSkipFetchingRefs(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	gitOps := &fakeGitOps{
		commits: []Commit{{SHA: "abcdef123456", Subject: "Add feature"}},
	}
	r := New(config.Config{
		Queue: config.Queue{
			Remote: "private",
		},
		Community: config.Community{
			Remote: "private",
		},
		Workflow: config.Workflow{
			CommitRange: "47824259^..1660a7c4",
		},
	}, store, gitOps, &fakeClient{}, &fakeClient{}, &fakeClient{})
	r.SetSkipFetch(true)

	count, err := r.SyncQueue()
	if err != nil {
		t.Fatalf("SyncQueue returned error: %v", err)
	}

	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	if len(gitOps.refreshed) != 0 {
		t.Fatalf("unexpected fetch remotes: %#v", gitOps.refreshed)
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

func TestLocalGitOpsPushSingleCommitBranchReturnsEmptyCherryPick(t *testing.T) {
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

	writeRepoFile(t, dir, "feature.txt", "feature\n")
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-m", "feature")
	featureSHA := gitOutput(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "branch", "base")
	runGit(t, dir, "push", "origin", "base")

	_, err := LocalGitOps{Dir: dir}.PushSingleCommitBranch(Commit{SHA: featureSHA}, "mr-empty", "base", "origin")
	if !errors.Is(err, ErrEmptyCherryPick) {
		t.Fatalf("err = %v", err)
	}
}

func TestLocalGitOpsConfiguresAskpassCredentialEnvironment(t *testing.T) {
	cmd := exec.Command("git", "version")
	cleanup, err := LocalGitOps{
		Username:    "smileQiny",
		AccessToken: "token-secret",
	}.configureCredentialEnv(cmd)
	if err != nil {
		t.Fatalf("configureCredentialEnv returned error: %v", err)
	}
	defer cleanup()

	env := map[string]string{}
	for _, item := range cmd.Env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	if env["GIT_TERMINAL_PROMPT"] != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q", env["GIT_TERMINAL_PROMPT"])
	}
	if env["MR_QUEUE_GIT_USERNAME"] != "smileQiny" {
		t.Fatalf("username = %q", env["MR_QUEUE_GIT_USERNAME"])
	}
	if env["MR_QUEUE_GIT_PASSWORD"] != "token-secret" {
		t.Fatalf("password = %q", env["MR_QUEUE_GIT_PASSWORD"])
	}
	if env["GIT_ASKPASS"] == "" {
		t.Fatal("GIT_ASKPASS is empty")
	}
	if _, err := os.Stat(env["GIT_ASKPASS"]); err != nil {
		t.Fatalf("stat askpass: %v", err)
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

func boolPtr(value bool) *bool {
	return &value
}
