package state

import (
	"path/filepath"
	"testing"
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
