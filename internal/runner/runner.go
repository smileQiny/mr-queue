package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mr-queue/internal/config"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/state"
)

type Commit struct {
	SHA     string
	Subject string
	Body    string
}

type GitOps interface {
	RefreshRefs(remotes []string) error
	ListCommits(commitRange string) ([]Commit, error)
	PushSingleCommitBranch(commit Commit, branchName string, baseRef string, remote string) (string, error)
}

type PullClient interface {
	CreatePull(owner string, repo string, input gitcode.PullRequestInput) (gitcode.PullRequest, error)
	CommentPull(owner string, repo string, number int, body string) (gitcode.Comment, error)
	ReviewPull(owner string, repo string, number int) (gitcode.Review, error)
	MergePull(owner string, repo string, number int, input gitcode.MergeInput) (gitcode.MergeResult, error)
}

type Runner struct {
	cfg        config.Config
	store      *state.Store
	gitOps     GitOps
	submitter  PullClient
	reviewer   PullClient
	maintainer PullClient
}

func New(cfg config.Config, store *state.Store, gitOps GitOps, submitter PullClient, reviewer PullClient, maintainer PullClient) *Runner {
	return &Runner{
		cfg:        cfg,
		store:      store,
		gitOps:     gitOps,
		submitter:  submitter,
		reviewer:   reviewer,
		maintainer: maintainer,
	}
}

func (r *Runner) RunOnce() error {
	if r.store.Snapshot().Paused {
		return nil
	}
	if err := r.gitOps.RefreshRefs(r.remotesToRefresh()); err != nil {
		return err
	}
	commits, err := r.gitOps.ListCommits(r.cfg.Workflow.CommitRange)
	if err != nil {
		return err
	}
	for _, commit := range commits {
		if err := r.store.UpsertTask(commit.SHA, commit.Subject); err != nil {
			return err
		}
		task := r.store.Snapshot().Tasks[commit.SHA]
		if task.Status == state.StatusMerged {
			continue
		}
		if task.Status != "" && task.Status != state.StatusPending {
			continue
		}
		return r.process(commit)
	}
	return nil
}

func (r *Runner) RunLoop(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		delay = time.Second
	}
	for {
		before := r.store.Snapshot()
		if before.Paused {
			return nil
		}
		if err := r.RunOnce(); err != nil {
			return err
		}
		after := r.store.Snapshot()
		if sameSnapshotProgress(before, after) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (r *Runner) process(commit Commit) error {
	sha := commit.SHA
	if err := r.store.SetTaskStatus(sha, state.StatusRunning, ""); err != nil {
		return err
	}
	branch := branchName(r.privateBranchPrefix(), sha)
	if err := r.store.SetTaskBranch(sha, branch); err != nil {
		return err
	}
	if err := r.store.AppendLog(sha, "push", fmt.Sprintf("Pushing %s to %s", sha, branch)); err != nil {
		return err
	}
	mrCommitSHA, err := r.gitOps.PushSingleCommitBranch(commit, branch, r.queueBaseRef(), r.privateRemote())
	if err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if err := r.store.SetTaskMRCommit(sha, mrCommitSHA); err != nil {
		return err
	}
	if err := r.store.SetTaskStatus(sha, state.StatusPushed, ""); err != nil {
		return err
	}

	head := branch
	if r.privateHeadNamespace() != "" {
		head = r.privateHeadNamespace() + ":" + branch
	}
	pr, err := r.submitter.CreatePull(r.communityOwner(), r.communityRepo(), gitcode.PullRequestInput{
		Title: commit.Subject,
		Head:  head,
		Base:  r.communityBranch(),
		Body:  commit.Body,
	})
	if err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if err := r.store.SetTaskMR(sha, pr.Number, pr.HTMLURL); err != nil {
		return err
	}
	if err := r.store.SetTaskStatus(sha, state.StatusMROpen, ""); err != nil {
		return err
	}
	if err := r.store.AppendLog(sha, "mr", fmt.Sprintf("Created MR #%d", pr.Number)); err != nil {
		return err
	}

	if _, err := r.reviewer.CommentPull(r.communityOwner(), r.communityRepo(), pr.Number, r.cfg.Workflow.ReviewComment); err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if _, err := r.reviewer.ReviewPull(r.communityOwner(), r.communityRepo(), pr.Number); err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if err := r.store.SetTaskStatus(sha, state.StatusReviewed, ""); err != nil {
		return err
	}
	if err := r.store.AppendLog(sha, "review", "Submitted review comment and approval"); err != nil {
		return err
	}

	mergeResult, err := r.maintainer.MergePull(r.communityOwner(), r.communityRepo(), pr.Number, gitcode.MergeInput{MergeMethod: r.cfg.Workflow.MergeMethod})
	if err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if mergeResult.SHA != "" {
		if err := r.store.SetTaskCommunityCommit(sha, mergeResult.SHA); err != nil {
			return err
		}
	}
	if err := r.store.SetTaskStatus(sha, state.StatusMerged, ""); err != nil {
		return err
	}
	return r.store.AppendLog(sha, "merge", fmt.Sprintf("Merged MR #%d", pr.Number))
}

func (r *Runner) remotesToRefresh() []string {
	seen := map[string]bool{}
	var remotes []string
	for _, remote := range []string{r.cfg.Queue.Remote, r.cfg.Community.Remote} {
		if remote != "" && !seen[remote] {
			seen[remote] = true
			remotes = append(remotes, remote)
		}
	}
	return remotes
}

func (r *Runner) privateRemote() string {
	if r.cfg.Private.Remote != "" {
		return r.cfg.Private.Remote
	}
	return r.cfg.Source.Remote
}

func (r *Runner) privateBranchPrefix() string {
	if r.cfg.Private.BranchPrefix != "" {
		return r.cfg.Private.BranchPrefix
	}
	return r.cfg.Source.BranchPrefix
}

func (r *Runner) privateHeadNamespace() string {
	if r.cfg.Private.HeadNamespace != "" {
		return r.cfg.Private.HeadNamespace
	}
	return r.cfg.Source.HeadNamespace
}

func (r *Runner) queueBaseRef() string {
	if r.cfg.Queue.BaseRef != "" {
		return r.cfg.Queue.BaseRef
	}
	return r.cfg.Source.BaseRef
}

func (r *Runner) communityOwner() string {
	if r.cfg.Community.Owner != "" {
		return r.cfg.Community.Owner
	}
	return r.cfg.Target.Owner
}

func (r *Runner) communityRepo() string {
	if r.cfg.Community.Repo != "" {
		return r.cfg.Community.Repo
	}
	return r.cfg.Target.Repo
}

func (r *Runner) communityBranch() string {
	if r.cfg.Community.Branch != "" {
		return r.cfg.Community.Branch
	}
	return r.cfg.Target.Branch
}

func (r *Runner) fail(sha string, err error) error {
	if logErr := r.store.AppendLog(sha, "error", err.Error()); logErr != nil {
		return logErr
	}
	return r.store.SetTaskStatus(sha, state.StatusFailed, err.Error())
}

type LocalGitOps struct {
	Dir string
}

func (g LocalGitOps) RefreshRefs(remotes []string) error {
	for _, remote := range remotes {
		if strings.TrimSpace(remote) == "" {
			continue
		}
		if err := g.run("fetch", "--prune", remote); err != nil {
			return err
		}
	}
	return nil
}

func (g LocalGitOps) ListCommits(commitRange string) ([]Commit, error) {
	args := []string{"log", "--reverse", "--format=%H%x00%s%x00%B%x1e", commitRange}
	out, err := g.git(args...)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "ambiguous argument") {
			return nil, nil
		}
		return nil, err
	}
	raw := strings.TrimRight(out, "\x1e\n")
	if raw == "" {
		return nil, nil
	}
	records := strings.Split(raw, "\x1e")
	var commits []Commit
	for _, record := range records {
		record = strings.Trim(record, "\n")
		parts := strings.SplitN(record, "\x00", 3)
		if len(parts) < 2 {
			continue
		}
		body := ""
		if len(parts) == 3 {
			body = strings.TrimSpace(parts[2])
		}
		commits = append(commits, Commit{
			SHA:     strings.TrimSpace(parts[0]),
			Subject: strings.TrimSpace(parts[1]),
			Body:    body,
		})
	}
	return commits, nil
}

func (g LocalGitOps) PushSingleCommitBranch(commit Commit, branchName string, baseRef string, remote string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "mr-queue-worktree-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpBranch := branchName + "-tmp"
	if err := g.run("worktree", "add", "--detach", tmpDir, baseRef); err != nil {
		return "", err
	}

	worktree := LocalGitOps{Dir: tmpDir}
	if err := worktree.run("checkout", "-B", tmpBranch); err != nil {
		_ = g.run("worktree", "remove", "--force", tmpDir)
		return "", err
	}
	if err := worktree.run("cherry-pick", commit.SHA); err != nil {
		_ = g.run("worktree", "remove", "--force", tmpDir)
		return "", err
	}
	mrCommitSHA, err := worktree.git("rev-parse", "HEAD")
	if err != nil {
		_ = g.run("worktree", "remove", "--force", tmpDir)
		return "", err
	}
	mrCommitSHA = strings.TrimSpace(mrCommitSHA)
	if err := worktree.run("push", remote, "HEAD:"+branchName, "--force-with-lease"); err != nil {
		_ = g.run("worktree", "remove", "--force", tmpDir)
		return "", err
	}
	if err := g.run("worktree", "remove", "--force", tmpDir); err != nil {
		return "", err
	}
	if err := g.run("branch", "-D", tmpBranch); err != nil {
		return "", err
	}
	return mrCommitSHA, nil
}

func (g LocalGitOps) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if g.Dir != "" {
		cmd.Dir = filepath.Clean(g.Dir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

func (g LocalGitOps) run(args ...string) error {
	_, err := g.git(args...)
	return err
}

func branchName(prefix string, sha string) string {
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	return strings.TrimRight(prefix, "-") + "-" + short
}

func sameSnapshotProgress(before state.Snapshot, after state.Snapshot) bool {
	if len(before.Tasks) != len(after.Tasks) {
		return false
	}
	for sha, beforeTask := range before.Tasks {
		afterTask, ok := after.Tasks[sha]
		if !ok {
			return false
		}
		if beforeTask.Status != afterTask.Status ||
			beforeTask.MRNumber != afterTask.MRNumber ||
			beforeTask.Branch != afterTask.Branch ||
			beforeTask.Error != afterTask.Error ||
			len(beforeTask.Logs) != len(afterTask.Logs) {
			return false
		}
	}
	return true
}
