package state

import (
	"path/filepath"
	"testing"

	"mr-queue/internal/config"
)

func TestTaskStatusAndLogsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := store.UpsertTask("abc123", "Add docs"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abc123", StatusMerged, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskMRCommit("abc123", "mr456"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskCommunityCommit("abc123", "community789"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendLog("abc123", "merge", "Merged PR #7"); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	snapshot := reopened.Snapshot()
	task := snapshot.Tasks["abc123"]
	if task.Subject != "Add docs" {
		t.Fatalf("subject = %q", task.Subject)
	}
	if task.Status != StatusMerged {
		t.Fatalf("status = %q", task.Status)
	}
	if task.MRCommitSHA != "mr456" {
		t.Fatalf("mr commit sha = %q", task.MRCommitSHA)
	}
	if task.CommunityCommitSHA != "community789" {
		t.Fatalf("community commit sha = %q", task.CommunityCommitSHA)
	}
	if len(task.Logs) != 1 || task.Logs[0].Step != "merge" {
		t.Fatalf("logs = %#v", task.Logs)
	}
}

func TestRetryFailedTaskMarksPendingAndKeepsHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := store.UpsertTask("abc123", "Add docs"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("abc123", StatusFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := store.RetryTask("abc123"); err != nil {
		t.Fatal(err)
	}

	task := store.Snapshot().Tasks["abc123"]
	if task.Status != StatusPending {
		t.Fatalf("status = %q", task.Status)
	}
	if task.Error != "" {
		t.Fatalf("error = %q", task.Error)
	}
	if task.Attempts != 1 {
		t.Fatalf("attempts = %d", task.Attempts)
	}
}

func TestQueueIndexPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := store.UpsertTaskAt("abc123", "Add docs", 7); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	task := reopened.Snapshot().Tasks["abc123"]
	if task.QueueIndex != 7 {
		t.Fatalf("queue index = %d", task.QueueIndex)
	}
}

func TestReplaceQueueTasksRemovesTasksOutsideCurrentQueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := store.UpsertTaskAt("old", "Old task", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("old", StatusFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertTaskAt("keep", "Keep old subject", 7); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskStatus("keep", StatusMerged, ""); err != nil {
		t.Fatal(err)
	}

	if err := store.ReplaceQueueTasks([]QueueTask{
		{SHA: "keep", Subject: "Keep new subject"},
		{SHA: "new", Subject: "New task"},
	}); err != nil {
		t.Fatal(err)
	}

	tasks := store.Snapshot().Tasks
	if _, ok := tasks["old"]; ok {
		t.Fatalf("old task was not removed: %#v", tasks["old"])
	}
	keep := tasks["keep"]
	if keep.Status != StatusMerged {
		t.Fatalf("keep status = %q", keep.Status)
	}
	if keep.Subject != "Keep new subject" {
		t.Fatalf("keep subject = %q", keep.Subject)
	}
	if keep.QueueIndex != 0 {
		t.Fatalf("keep queue index = %d", keep.QueueIndex)
	}
	newTask := tasks["new"]
	if newTask.Status != StatusPending {
		t.Fatalf("new status = %q", newTask.Status)
	}
	if newTask.QueueIndex != 1 {
		t.Fatalf("new queue index = %d", newTask.QueueIndex)
	}
}

func TestReplaceQueueTasksForConfigRecordsConfigScopeAndTaskContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	cfg := config.Config{
		Provider: "atomgit",
		Source: config.Source{
			Repo:   "atomgit.com/w-xxxx/track-system",
			Branch: "w-xxxx/track-system-1.2.0",
		},
		Target: config.Target{
			Repo:   "atomgit.com/openeuler/track-system",
			Branch: "master",
		},
		Queue: config.Queue{
			Remote:    "mrq-atomgit-com-w-xxxx-track-system",
			RemoteURL: "https://atomgit.com/w-xxxx/track-system.git",
			Branch:    "w-xxxx/track-system-1.2.0",
			BaseRef:   "mrq-atomgit-com-openeuler-track-system/master",
		},
		Private: config.Private{
			Remote:         "mrq-atomgit-com-w-xxxx-track-system",
			RemoteURL:      "https://atomgit.com/w-xxxx/track-system.git",
			BranchPrefix:   "track-system",
			BranchTemplate: "{prefix}-{title_or_sha12}",
			HeadNamespace:  "w-xxxx",
		},
		Community: config.Community{
			Remote:    "mrq-atomgit-com-openeuler-track-system",
			RemoteURL: "https://atomgit.com/openeuler/track-system.git",
			Owner:     "openeuler",
			Repo:      "track-system",
			Branch:    "master",
		},
		Workflow: config.Workflow{
			CommitRange:   "mrq-atomgit-com-openeuler-track-system/master..mrq-atomgit-com-w-xxxx-track-system/w-xxxx/track-system-1.2.0",
			MergeMethod:   "external",
			ReviewComment: "/lgtm\n/approve\n",
		},
		Auth: config.Auth{
			Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "sub-token"},
			Reviewer:  config.Credential{TokenEnv: "GITCODE_REVIEWER_TOKEN", Token: "review-token"},
		},
	}

	if err := store.ReplaceQueueTasksForConfig(cfg, []QueueTask{
		{SHA: "abc123", Subject: "First task"},
		{SHA: "def456", Subject: "Second task"},
	}); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	snapshot := reopened.Snapshot()
	if snapshot.ActiveConfigVersionID == "" {
		t.Fatal("active config version id is empty")
	}
	if snapshot.ActiveScopeID == "" {
		t.Fatal("active scope id is empty")
	}
	version := snapshot.ConfigVersions[snapshot.ActiveConfigVersionID]
	if version.SourceRepo != "atomgit.com/w-xxxx/track-system" {
		t.Fatalf("source repo = %q", version.SourceRepo)
	}
	if version.TargetRepo != "atomgit.com/openeuler/track-system" {
		t.Fatalf("target repo = %q", version.TargetRepo)
	}
	if version.ResolvedCommitRange != cfg.Workflow.CommitRange {
		t.Fatalf("resolved commit range = %q", version.ResolvedCommitRange)
	}
	if version.SubmitterTokenEnv != "GITCODE_SUBMITTER_TOKEN" {
		t.Fatalf("submitter token env = %q", version.SubmitterTokenEnv)
	}
	if version.SafeConfigJSON == "" || version.ResolvedConfigJSON == "" {
		t.Fatalf("config json was not recorded: %#v", version)
	}
	if version.Active != true {
		t.Fatalf("active = %v", version.Active)
	}
	scope := snapshot.TaskScopes[snapshot.ActiveScopeID]
	if scope.ConfigVersionID != version.ID {
		t.Fatalf("scope config version id = %q, want %q", scope.ConfigVersionID, version.ID)
	}
	if scope.ResolvedCommitRange != cfg.Workflow.CommitRange {
		t.Fatalf("scope commit range = %q", scope.ResolvedCommitRange)
	}
	if scope.CommitCount != 2 {
		t.Fatalf("scope commit count = %d", scope.CommitCount)
	}
	task := snapshot.Tasks["abc123"]
	if task.ConfigVersionID != version.ID || task.ScopeID != scope.ID {
		t.Fatalf("task context = config %q scope %q, want config %q scope %q", task.ConfigVersionID, task.ScopeID, version.ID, scope.ID)
	}
	if task.SourceRepo != cfg.Source.Repo || task.TargetBranch != cfg.Target.Branch || task.CommitRange != cfg.Workflow.CommitRange {
		t.Fatalf("task config context = %#v", task)
	}
	if _, ok := snapshot.TaskRecords[scope.ID+":abc123"]; !ok {
		t.Fatalf("task record was not keyed by scope and sha: %#v", snapshot.TaskRecords)
	}
}

func TestSelectScopeRestoresTasksForThatScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	first := config.Config{
		Source:   config.Source{Repo: "atomgit.com/source/project", Branch: "feature-a"},
		Target:   config.Target{Repo: "atomgit.com/target/project", Branch: "master"},
		Workflow: config.Workflow{CommitRange: "target/master..source/feature-a"},
		Private:  config.Private{RemoteURL: "https://atomgit.com/source/project.git", BranchPrefix: "feat", BranchTemplate: "{prefix}-{sha12}"},
	}
	second := first
	second.Source.Branch = "feature-b"
	second.Workflow.CommitRange = "target/master..source/feature-b"

	if err := store.ReplaceQueueTasksForConfig(first, []QueueTask{{SHA: "aaa111", Subject: "First"}}); err != nil {
		t.Fatal(err)
	}
	firstScope := store.Snapshot().ActiveScopeID
	if err := store.SetTaskStatus("aaa111", StatusMerged, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceQueueTasksForConfig(second, []QueueTask{{SHA: "bbb222", Subject: "Second"}}); err != nil {
		t.Fatal(err)
	}
	secondScope := store.Snapshot().ActiveScopeID
	if firstScope == secondScope {
		t.Fatalf("scope ids should differ: %q", firstScope)
	}

	if err := store.SelectScope(firstScope); err != nil {
		t.Fatal(err)
	}
	selected := store.Snapshot()
	if selected.ActiveScopeID != firstScope {
		t.Fatalf("active scope = %q, want %q", selected.ActiveScopeID, firstScope)
	}
	if selected.ActiveConfigVersionID != selected.TaskScopes[firstScope].ConfigVersionID {
		t.Fatalf("active config = %q, scope config = %q", selected.ActiveConfigVersionID, selected.TaskScopes[firstScope].ConfigVersionID)
	}
	if _, ok := selected.Tasks["bbb222"]; ok {
		t.Fatalf("second scope task still active: %#v", selected.Tasks)
	}
	task := selected.Tasks["aaa111"]
	if task.Status != StatusMerged {
		t.Fatalf("restored status = %q", task.Status)
	}

	if err := store.SelectScope(secondScope); err != nil {
		t.Fatal(err)
	}
	selected = store.Snapshot()
	if _, ok := selected.Tasks["aaa111"]; ok {
		t.Fatalf("first scope task still active after selecting second: %#v", selected.Tasks)
	}
	if selected.Tasks["bbb222"].Status != StatusPending {
		t.Fatalf("second task status = %q", selected.Tasks["bbb222"].Status)
	}
}

func TestTaskScopeIdentityIncludesCommitRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	first := config.Config{
		Source:   config.Source{Repo: "atomgit.com/source/project", Branch: "feature-a"},
		Target:   config.Target{Repo: "atomgit.com/target/project", Branch: "master"},
		Workflow: config.Workflow{CommitRange: "aaa111^..bbb222"},
		Private:  config.Private{RemoteURL: "https://atomgit.com/source/project.git", BranchPrefix: "feat", BranchTemplate: "{prefix}-{sha12}"},
	}
	second := first
	second.Workflow.CommitRange = "ccc333^..ddd444"

	if err := store.ReplaceQueueTasksForConfig(first, []QueueTask{
		{SHA: "111aaa", Subject: "First"},
		{SHA: "222bbb", Subject: "Second"},
	}); err != nil {
		t.Fatal(err)
	}
	firstScope := store.Snapshot().ActiveScopeID
	if err := store.ReplaceQueueTasksForConfig(second, []QueueTask{
		{SHA: "333ccc", Subject: "Third"},
		{SHA: "444ddd", Subject: "Fourth"},
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := store.Snapshot()
	secondScope := snapshot.ActiveScopeID

	if firstScope == secondScope {
		t.Fatalf("scope id should differ when commit range differs: %q", firstScope)
	}
	if len(snapshot.TaskScopes) != 2 {
		t.Fatalf("task scopes = %d, want 2: %#v", len(snapshot.TaskScopes), snapshot.TaskScopes)
	}
	if snapshot.TaskScopes[firstScope].CommitCount != snapshot.TaskScopes[secondScope].CommitCount {
		t.Fatalf("test setup should use same commit count: %d vs %d", snapshot.TaskScopes[firstScope].CommitCount, snapshot.TaskScopes[secondScope].CommitCount)
	}
	if snapshot.TaskScopes[firstScope].ResolvedCommitRange == snapshot.TaskScopes[secondScope].ResolvedCommitRange {
		t.Fatalf("scope ranges should differ: %#v", snapshot.TaskScopes)
	}
}
