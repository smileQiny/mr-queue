package runner

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mr-queue/internal/config"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/state"
)

var ErrEmptyCherryPick = errors.New("empty cherry-pick")

type Commit struct {
	SHA     string
	Subject string
	Body    string
}

type GitOps interface {
	RefreshRefs(remotes []string) error
	ListCommits(commitRange string) ([]Commit, error)
	IsCommitAlreadyApplied(commit Commit, baseRef string) (bool, error)
	PushSingleCommitBranch(commit Commit, branchName string, baseRef string, remote string) (string, error)
}

type PullClient interface {
	CreatePull(owner string, repo string, input gitcode.PullRequestInput) (gitcode.PullRequest, error)
	GetPull(owner string, repo string, number int) (gitcode.PullRequest, error)
	CommentPull(owner string, repo string, number int, body string) (gitcode.Comment, error)
	ListPullComments(owner string, repo string, number int) ([]gitcode.Comment, error)
	ReviewPull(owner string, repo string, number int) (gitcode.Review, error)
	MergePull(owner string, repo string, number int, input gitcode.MergeInput) (gitcode.MergeResult, error)
}

type LoopOptions struct {
	MinDelay          time.Duration
	MaxDelay          time.Duration
	WaitCheckDelayMin time.Duration
	WaitCheckDelayMax time.Duration
	NextPRDelayMin    time.Duration
	NextPRDelayMax    time.Duration
	MaxMergedCommits  int
	WorkWindowStart   string
	WorkWindowEnd     string
	Now               func() time.Time
	Sleep             func(context.Context, time.Duration) error
	OnProgress        func(LoopProgress)
}

type LoopResult struct {
	StopReason  string
	MergedCount int
}

type LoopProgress struct {
	Message      string
	Delay        time.Duration
	MergedCount  int
	TargetMerged int
}

type Runner struct {
	cfg        config.Config
	store      *state.Store
	gitOps     GitOps
	submitter  PullClient
	reviewer   PullClient
	maintainer PullClient
	skipFetch  bool
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

func (r *Runner) SetSkipFetch(skipFetch bool) {
	r.skipFetch = skipFetch
}

func (r *Runner) RunOnce() error {
	if r.store.Snapshot().Paused {
		return nil
	}
	if err := r.refreshRefs(); err != nil {
		return err
	}
	commits, err := r.gitOps.ListCommits(r.cfg.Workflow.CommitRange)
	if err != nil {
		return err
	}
	queueTasks := queueTasksFromCommits(commits)
	if err := r.store.ReplaceQueueTasksForConfig(r.cfg, queueTasks); err != nil {
		return err
	}
	for _, commit := range commits {
		task := r.store.Snapshot().Tasks[commit.SHA]
		if task.Status == state.StatusMerged || task.Status == state.StatusSkipped {
			continue
		}
		if task.Status != "" && task.Status != state.StatusPending && task.Status != state.StatusWaitingRequiredComment && task.Status != state.StatusWaitingExternalMerge {
			continue
		}
		return r.process(commit)
	}
	return nil
}

func (r *Runner) SyncQueue() (int, error) {
	if err := r.refreshRefs(); err != nil {
		return 0, err
	}
	commits, err := r.gitOps.ListCommits(r.cfg.Workflow.CommitRange)
	if err != nil {
		return 0, err
	}
	queueTasks := queueTasksFromCommits(commits)
	if err := r.store.ReplaceQueueTasksForConfig(r.cfg, queueTasks); err != nil {
		return 0, err
	}
	return len(commits), nil
}

func queueTasksFromCommits(commits []Commit) []state.QueueTask {
	queueTasks := make([]state.QueueTask, 0, len(commits))
	for _, commit := range commits {
		queueTasks = append(queueTasks, state.QueueTask{SHA: commit.SHA, Subject: commit.Subject})
	}
	return queueTasks
}

func (r *Runner) RefreshWaiting() error {
	snapshot := r.store.Snapshot()
	for _, task := range snapshot.Tasks {
		switch task.Status {
		case state.StatusWaitingExternalMerge:
			if err := r.checkExternalMerge(task.SHA, task.MRNumber); err != nil {
				return err
			}
		case state.StatusWaitingRequiredComment:
			if task.MRNumber == 0 || r.cfg.Workflow.RequiredCommentText == "" {
				continue
			}
			ok, err := r.hasRequiredComment(task.MRNumber)
			if err != nil {
				_ = r.fail(task.SHA, err)
				return err
			}
			if !ok {
				continue
			}
			commit := Commit{SHA: task.SHA, Subject: task.Subject}
			if err := r.process(commit); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Runner) refreshRefs() error {
	if r.skipFetch {
		return nil
	}
	return r.gitOps.RefreshRefs(r.remotesToRefresh())
}

func (r *Runner) RunLoop(ctx context.Context, delay time.Duration) error {
	_, err := r.RunLoopRange(ctx, delay, delay)
	return err
}

func (r *Runner) RunLoopRange(ctx context.Context, minDelay time.Duration, maxDelay time.Duration) (LoopResult, error) {
	return r.RunLoopWithOptions(ctx, LoopOptions{
		MinDelay:          minDelay,
		MaxDelay:          maxDelay,
		WaitCheckDelayMin: minDelay,
		WaitCheckDelayMax: maxDelay,
	})
}

func (r *Runner) RunLoopWithOptions(ctx context.Context, options LoopOptions) (LoopResult, error) {
	waitMinDelay, waitMaxDelay := normalizeDelayRange(options.WaitCheckDelayMin, options.WaitCheckDelayMax, time.Second, time.Second)
	nextMinDelay := options.NextPRDelayMin
	nextMaxDelay := options.NextPRDelayMax
	if nextMinDelay <= 0 {
		nextMinDelay = options.MinDelay
	}
	if nextMaxDelay <= 0 {
		nextMaxDelay = options.MaxDelay
	}
	nextMinDelay, nextMaxDelay = normalizeDelayRange(nextMinDelay, nextMaxDelay, time.Second, time.Second)
	now := options.Now
	if now == nil {
		now = time.Now
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	eligibleForLimit := mergeLimitEligibleTasks(r.store.Snapshot())
	mergedCount := 0
	for {
		before := r.store.Snapshot()
		if before.Paused {
			return LoopResult{StopReason: "queue paused", MergedCount: mergedCount}, nil
		}
		inWindow, err := inWorkWindow(now(), options.WorkWindowStart, options.WorkWindowEnd)
		if err != nil {
			return LoopResult{MergedCount: mergedCount}, err
		}
		if !inWindow {
			return LoopResult{StopReason: "outside work window", MergedCount: mergedCount}, nil
		}
		if err := r.RunOnce(); err != nil {
			return LoopResult{MergedCount: mergedCount}, err
		}
		after := r.store.Snapshot()
		markNewMergeLimitEligibleTasks(before, after, eligibleForLimit)
		mergedThisIteration := newlyMergedEligibleCount(before, after, eligibleForLimit)
		mergedCount += mergedThisIteration
		if options.MaxMergedCommits > 0 && mergedCount >= options.MaxMergedCommits {
			return LoopResult{
				StopReason:  fmt.Sprintf("reached max merged commits: %d/%d", mergedCount, options.MaxMergedCommits),
				MergedCount: mergedCount,
			}, nil
		}
		if sameSnapshotProgress(before, after) && !hasWaitingTask(after) {
			return LoopResult{StopReason: "no pending or waiting tasks progressed", MergedCount: mergedCount}, nil
		}
		delay := nextLoopDelay(before, after, waitMinDelay, waitMaxDelay, nextMinDelay, nextMaxDelay)
		if options.OnProgress != nil && mergedThisIteration > 0 {
			progress := LoopProgress{
				Delay:        delay,
				MergedCount:  mergedCount,
				TargetMerged: options.MaxMergedCommits,
			}
			progress.Message = loopProgressMessage(progress)
			options.OnProgress(progress)
		}
		if err := sleep(ctx, delay); err != nil {
			return LoopResult{StopReason: "stopped by user", MergedCount: mergedCount}, err
		}
	}
}

func loopProgressMessage(progress LoopProgress) string {
	target := "不限"
	if progress.TargetMerged > 0 {
		target = fmt.Sprintf("%d", progress.TargetMerged)
	}
	return fmt.Sprintf("本轮计划合入 %s 个 PR，当前已合入 %d 个；下个 PR 将在 %s 后启动", target, progress.MergedCount, progress.Delay)
}

func normalizeDelayRange(minDelay time.Duration, maxDelay time.Duration, defaultMin time.Duration, defaultMax time.Duration) (time.Duration, time.Duration) {
	if minDelay <= 0 {
		minDelay = defaultMin
	}
	if maxDelay <= 0 {
		maxDelay = defaultMax
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	return minDelay, maxDelay
}

func nextLoopDelay(before state.Snapshot, after state.Snapshot, waitMinDelay time.Duration, waitMaxDelay time.Duration, nextMinDelay time.Duration, nextMaxDelay time.Duration) time.Duration {
	if newlySkippedCount(before, after) > 0 {
		return 0
	}
	if newlyMergedCount(before, after) > 0 {
		return randomDelay(nextMinDelay, nextMaxDelay)
	}
	if hasWaitingTask(after) {
		return randomDelay(waitMinDelay, waitMaxDelay)
	}
	return randomDelay(nextMinDelay, nextMaxDelay)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

func mergeLimitEligibleTasks(snapshot state.Snapshot) map[string]bool {
	eligible := map[string]bool{}
	for sha, task := range snapshot.Tasks {
		if task.Status == "" || task.Status == state.StatusPending {
			eligible[sha] = true
		}
	}
	return eligible
}

func markNewMergeLimitEligibleTasks(before state.Snapshot, after state.Snapshot, eligible map[string]bool) {
	for sha, afterTask := range after.Tasks {
		if eligible[sha] {
			continue
		}
		if _, ok := before.Tasks[sha]; ok {
			continue
		}
		if afterTask.Status != state.StatusSkipped && afterTask.Status != state.StatusFailed {
			eligible[sha] = true
		}
	}
}

func newlyMergedEligibleCount(before state.Snapshot, after state.Snapshot, eligible map[string]bool) int {
	count := 0
	for sha, afterTask := range after.Tasks {
		if !eligible[sha] || afterTask.Status != state.StatusMerged {
			continue
		}
		beforeTask, ok := before.Tasks[sha]
		if !ok || beforeTask.Status != state.StatusMerged {
			count++
		}
	}
	return count
}

func inWorkWindow(now time.Time, start string, end string) (bool, error) {
	if strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
		return true, nil
	}
	startMinute, err := parseClockMinute(start)
	if err != nil {
		return false, fmt.Errorf("parse work_window_start: %w", err)
	}
	endMinute, err := parseClockMinute(end)
	if err != nil {
		return false, fmt.Errorf("parse work_window_end: %w", err)
	}
	currentMinute := now.Hour()*60 + now.Minute()
	if startMinute <= endMinute {
		return currentMinute >= startMinute && currentMinute <= endMinute, nil
	}
	return currentMinute >= startMinute || currentMinute <= endMinute, nil
}

func parseClockMinute(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("expected HH:MM")
	}
	return hour*60 + minute, nil
}

func newlyMergedCount(before state.Snapshot, after state.Snapshot) int {
	count := 0
	for sha, afterTask := range after.Tasks {
		if afterTask.Status != state.StatusMerged {
			continue
		}
		beforeTask, ok := before.Tasks[sha]
		if !ok || beforeTask.Status != state.StatusMerged {
			count++
		}
	}
	return count
}

func newlySkippedCount(before state.Snapshot, after state.Snapshot) int {
	count := 0
	for sha, afterTask := range after.Tasks {
		if afterTask.Status != state.StatusSkipped {
			continue
		}
		beforeTask, ok := before.Tasks[sha]
		if !ok || beforeTask.Status != state.StatusSkipped {
			count++
		}
	}
	return count
}

func hasWaitingTask(snapshot state.Snapshot) bool {
	for _, task := range snapshot.Tasks {
		if task.Status == state.StatusWaitingRequiredComment || task.Status == state.StatusWaitingExternalMerge {
			return true
		}
	}
	return false
}

func randomDelay(minDelay time.Duration, maxDelay time.Duration) time.Duration {
	if maxDelay <= minDelay {
		return minDelay
	}
	return minDelay + time.Duration(rand.Int64N(int64(maxDelay-minDelay)+1))
}

func (r *Runner) process(commit Commit) error {
	sha := commit.SHA
	task := r.store.Snapshot().Tasks[sha]
	if task.Status == state.StatusWaitingExternalMerge {
		return r.checkExternalMerge(sha, task.MRNumber)
	}
	if err := r.store.SetTaskStatus(sha, state.StatusRunning, ""); err != nil {
		return err
	}
	task = r.store.Snapshot().Tasks[sha]
	prNumber := task.MRNumber
	if prNumber == 0 {
		branch := task.Branch
		if branch == "" {
			branch = r.branchName(commit)
		}
		if task.MRCommitSHA == "" {
			applied, err := r.gitOps.IsCommitAlreadyApplied(commit, r.queueBaseRef())
			if err != nil {
				_ = r.fail(sha, err)
				return err
			}
			if applied {
				if err := r.store.SetTaskBranch(sha, branch); err != nil {
					return err
				}
				if err := r.store.SetTaskStatus(sha, state.StatusSkipped, ""); err != nil {
					return err
				}
				return r.store.AppendLog(sha, "skip", "Patch already exists on target base; skipped already-applied commit")
			}
			if err := r.store.SetTaskBranch(sha, branch); err != nil {
				return err
			}
		}
		if task.MRCommitSHA == "" {
			if err := r.store.AppendLog(sha, "push", fmt.Sprintf("Pushing %s to %s", sha, branch)); err != nil {
				return err
			}
			mrCommitSHA, err := r.gitOps.PushSingleCommitBranch(commit, branch, r.queueBaseRef(), r.privateRemote())
			if err != nil {
				if errors.Is(err, ErrEmptyCherryPick) {
					if err := r.store.SetTaskStatus(sha, state.StatusSkipped, ""); err != nil {
						return err
					}
					return r.store.AppendLog(sha, "skip", "Patch already exists on target base; skipped empty cherry-pick")
				}
				_ = r.fail(sha, err)
				return err
			}
			if err := r.store.SetTaskMRCommit(sha, mrCommitSHA); err != nil {
				return err
			}
			if err := r.store.SetTaskStatus(sha, state.StatusPushed, ""); err != nil {
				return err
			}
		}

		head := branch
		if r.privateHeadNamespace() != "" && r.privateHeadNamespace() != r.communityOwner() {
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
		prNumber = pr.Number
		if err := r.store.SetTaskMR(sha, pr.Number, pr.HTMLURL); err != nil {
			return err
		}
		if err := r.store.SetTaskStatus(sha, state.StatusMROpen, ""); err != nil {
			return err
		}
		if err := r.store.AppendLog(sha, "mr", fmt.Sprintf("Created MR #%d", pr.Number)); err != nil {
			return err
		}
	}

	if r.cfg.Workflow.RequiredCommentText != "" {
		ok, err := r.hasRequiredComment(prNumber)
		if err != nil {
			_ = r.fail(sha, err)
			return err
		}
		if !ok {
			if task.Status != state.StatusWaitingRequiredComment {
				if err := r.store.SetTaskStatus(sha, state.StatusWaitingRequiredComment, ""); err != nil {
					return err
				}
				if err := r.store.AppendLog(sha, "wait", "Waiting for required comment: "+r.cfg.Workflow.RequiredCommentText); err != nil {
					return err
				}
			}
			return nil
		}
		if task.Status == state.StatusWaitingRequiredComment {
			if err := r.store.AppendLog(sha, "wait", "Required comment found: "+r.cfg.Workflow.RequiredCommentText); err != nil {
				return err
			}
		}
	}

	if _, err := r.reviewer.CommentPull(r.communityOwner(), r.communityRepo(), prNumber, r.cfg.Workflow.ReviewComment); err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if r.cfg.Workflow.ShouldApprove() {
		if _, err := r.reviewer.ReviewPull(r.communityOwner(), r.communityRepo(), prNumber); err != nil {
			if r.cfg.Workflow.WarnOnApprovalFailure() {
				if logErr := r.store.AppendLog(sha, "approval", "Approval was rejected by the platform; continuing because approval_failure_mode=warn"); logErr != nil {
					return logErr
				}
			} else {
				_ = r.fail(sha, err)
				return err
			}
		}
	}
	if err := r.store.SetTaskStatus(sha, state.StatusReviewed, ""); err != nil {
		return err
	}
	reviewLog := "Submitted review comment"
	if r.cfg.Workflow.ShouldApprove() {
		reviewLog += " and approval"
	}
	if err := r.store.AppendLog(sha, "review", reviewLog); err != nil {
		return err
	}
	if r.cfg.Workflow.MergeMethod == "external" {
		if err := r.store.SetTaskStatus(sha, state.StatusWaitingExternalMerge, ""); err != nil {
			return err
		}
		return r.store.AppendLog(sha, "merge", fmt.Sprintf("Waiting for external merge of MR #%d", prNumber))
	}

	mergeResult, err := r.maintainer.MergePull(r.communityOwner(), r.communityRepo(), prNumber, gitcode.MergeInput{MergeMethod: r.cfg.Workflow.MergeMethod})
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
	return r.store.AppendLog(sha, "merge", fmt.Sprintf("Merged MR #%d", prNumber))
}

func (r *Runner) hasRequiredComment(prNumber int) (bool, error) {
	comments, err := r.reviewer.ListPullComments(r.communityOwner(), r.communityRepo(), prNumber)
	if err != nil {
		return false, err
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, r.cfg.Workflow.RequiredCommentText) {
			return true, nil
		}
	}
	return false, nil
}

func (r *Runner) checkExternalMerge(sha string, prNumber int) error {
	pr, err := r.reviewer.GetPull(r.communityOwner(), r.communityRepo(), prNumber)
	if err != nil {
		_ = r.fail(sha, err)
		return err
	}
	if !pr.Merged && pr.State != "merged" {
		return nil
	}
	mergeSHA := pr.MergeCommitSHA
	if mergeSHA == "" {
		mergeSHA = pr.HeadSHA
	}
	if mergeSHA != "" {
		if err := r.store.SetTaskCommunityCommit(sha, mergeSHA); err != nil {
			return err
		}
	}
	if err := r.store.SetTaskStatus(sha, state.StatusMerged, ""); err != nil {
		return err
	}
	return r.store.AppendLog(sha, "merge", fmt.Sprintf("External merge completed for MR #%d", prNumber))
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

func (r *Runner) privateBranchTemplate() string {
	if r.cfg.Private.BranchTemplate != "" {
		return r.cfg.Private.BranchTemplate
	}
	if r.cfg.Source.BranchTemplate != "" {
		return r.cfg.Source.BranchTemplate
	}
	return "{prefix}-{sha12}"
}

func (r *Runner) branchName(commit Commit) string {
	return branchName(r.privateBranchTemplate(), r.privateBranchPrefix(), commit)
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
	Dir            string
	Username       string
	AccessToken    string
	ManagedRemotes map[string]string
}

func (g LocalGitOps) RefreshRefs(remotes []string) error {
	for _, remote := range remotes {
		if strings.TrimSpace(remote) == "" {
			continue
		}
		if err := g.ensureManagedRemote(remote); err != nil {
			return err
		}
		if err := g.run("fetch", "--prune", remote); err != nil {
			return err
		}
	}
	return nil
}

func (g LocalGitOps) ensureManagedRemote(remote string) error {
	remoteURL := strings.TrimSpace(g.ManagedRemotes[remote])
	if remoteURL == "" {
		return nil
	}
	currentURL, err := g.git("remote", "get-url", remote)
	if err != nil {
		if strings.Contains(err.Error(), "No such remote") || strings.Contains(err.Error(), "No such remote '") {
			return g.run("remote", "add", remote, remoteURL)
		}
		return err
	}
	if strings.TrimSpace(currentURL) == remoteURL {
		return nil
	}
	return g.run("remote", "set-url", remote, remoteURL)
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

func (g LocalGitOps) IsCommitAlreadyApplied(commit Commit, baseRef string) (bool, error) {
	countOut, err := g.git("rev-list", "--count", baseRef+".."+commit.SHA)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(countOut) == "0" {
		return true, nil
	}
	cherryOut, err := g.git("cherry", baseRef, commit.SHA)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(cherryOut), "\n") {
		line = strings.TrimSpace(line)
		prefix, cherrySHA, ok := strings.Cut(line, " ")
		if ok && prefix == "-" && strings.TrimSpace(cherrySHA) == commit.SHA {
			return true, nil
		}
	}
	return false, nil
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

	worktree := LocalGitOps{Dir: tmpDir, Username: g.Username, AccessToken: g.AccessToken}
	if err := worktree.run("checkout", "-B", tmpBranch); err != nil {
		_ = g.run("worktree", "remove", "--force", tmpDir)
		return "", err
	}
	if err := worktree.run("cherry-pick", commit.SHA); err != nil {
		if isEmptyCherryPickError(err) {
			_ = worktree.run("cherry-pick", "--abort")
			_ = g.run("worktree", "remove", "--force", tmpDir)
			return "", ErrEmptyCherryPick
		}
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

func isEmptyCherryPickError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "previous cherry-pick is now empty") ||
		strings.Contains(text, "nothing to commit, working tree clean")
}

func (g LocalGitOps) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if g.Dir != "" {
		cmd.Dir = filepath.Clean(g.Dir)
	}
	cleanup, err := g.configureCredentialEnv(cmd)
	if err != nil {
		return "", err
	}
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

func (g LocalGitOps) GitForDoctor(args ...string) (string, error) {
	return g.git(args...)
}

func (g LocalGitOps) configureCredentialEnv(cmd *exec.Cmd) (func(), error) {
	if g.AccessToken == "" {
		return func() {}, nil
	}
	username := strings.TrimSpace(g.Username)
	if username == "" {
		username = "git"
	}
	dir, err := os.MkdirTemp("", "mr-queue-askpass-*")
	if err != nil {
		return nil, err
	}
	askpass := filepath.Join(dir, "askpass.sh")
	body := `#!/bin/sh
case "$1" in
*Username*) printf '%s\n' "$MR_QUEUE_GIT_USERNAME" ;;
*) printf '%s\n' "$MR_QUEUE_GIT_PASSWORD" ;;
esac
`
	if err := os.WriteFile(askpass, []byte(body), 0700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	cmd.Env = append(os.Environ(),
		"GIT_ASKPASS="+askpass,
		"GIT_TERMINAL_PROMPT=0",
		"MR_QUEUE_GIT_USERNAME="+username,
		"MR_QUEUE_GIT_PASSWORD="+g.AccessToken,
	)
	return func() { _ = os.RemoveAll(dir) }, nil
}

func (g LocalGitOps) run(args ...string) error {
	_, err := g.git(args...)
	return err
}

func branchName(template string, prefix string, commit Commit) string {
	short := shortSHA(commit.SHA)
	title := slugifyBranchPart(commit.Subject, 60)
	titleOrSHA := slugifyBranchPartOrEmpty(commit.Subject, 60)
	if titleOrSHA == "" {
		titleOrSHA = short
	}
	replacer := strings.NewReplacer(
		"{prefix}", slugifyBranchPart(prefix, 40),
		"{title_or_sha12}", titleOrSHA,
		"{title}", title,
		"{sha12}", short,
		"{sha}", commit.SHA,
	)
	name := replacer.Replace(template)
	name = normalizeBranchName(name)
	if name == "" {
		name = "commit-" + short
	}
	return strings.Trim(name, "-./")
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func slugifyBranchPart(value string, maxLen int) string {
	return trimBranchPart(normalizeBranchName(value), maxLen)
}

func slugifyBranchPartOrEmpty(value string, maxLen int) string {
	return trimBranchPartOrEmpty(normalizeBranchName(value), maxLen)
}

func normalizeBranchName(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func trimBranchPart(value string, maxLen int) string {
	if value == "" {
		return "commit"
	}
	return trimBranchPartOrEmpty(value, maxLen)
}

func trimBranchPartOrEmpty(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	value = strings.TrimRight(value[:maxLen], "-")
	return value
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
