package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusPushed   = "pushed"
	StatusMROpen   = "mr_open"
	StatusReviewed = "reviewed"
	StatusMerged   = "merged"
	StatusFailed   = "failed"
)

type Store struct {
	path string
	mu   sync.Mutex
	data Snapshot
}

type Snapshot struct {
	Tasks     map[string]Task `json:"tasks"`
	Paused    bool            `json:"paused"`
	UpdatedAt string          `json:"updated_at"`
}

type Task struct {
	SHA                string     `json:"sha"`
	Subject            string     `json:"subject"`
	Status             string     `json:"status"`
	Branch             string     `json:"branch,omitempty"`
	MRCommitSHA        string     `json:"mr_commit_sha,omitempty"`
	CommunityCommitSHA string     `json:"community_commit_sha,omitempty"`
	MRNumber           int        `json:"mr_number,omitempty"`
	MRURL              string     `json:"mr_url,omitempty"`
	Error              string     `json:"error,omitempty"`
	Attempts           int        `json:"attempts"`
	CreatedAt          string     `json:"created_at"`
	UpdatedAt          string     `json:"updated_at"`
	Logs               []LogEntry `json:"logs"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Step    string `json:"step"`
	Message string `json:"message"`
}

func Open(path string) (*Store, error) {
	store := &Store{
		path: path,
		data: Snapshot{Tasks: map[string]Task{}},
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			return store, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	if len(body) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(body, &store.data); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if store.data.Tasks == nil {
		store.data.Tasks = map[string]Task{}
	}
	return store, nil
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyData := Snapshot{
		Tasks:     map[string]Task{},
		Paused:    s.data.Paused,
		UpdatedAt: s.data.UpdatedAt,
	}
	for sha, task := range s.data.Tasks {
		task.Logs = append([]LogEntry(nil), task.Logs...)
		copyData.Tasks[sha] = task
	}
	return copyData
}

func (s *Store) SetPaused(paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Paused = paused
	return s.saveLocked()
}

func (s *Store) UpsertTask(sha string, subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := timestamp()
	task, ok := s.data.Tasks[sha]
	if !ok {
		task = Task{
			SHA:       sha,
			Subject:   subject,
			Status:    StatusPending,
			CreatedAt: now,
		}
	}
	task.Subject = subject
	task.UpdatedAt = now
	s.data.Tasks[sha] = task
	return s.saveLocked()
}

func (s *Store) SetTaskStatus(sha string, status string, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.data.Tasks[sha]
	if !ok {
		return fmt.Errorf("task %s does not exist", sha)
	}
	task.Status = status
	task.Error = errText
	task.UpdatedAt = timestamp()
	s.data.Tasks[sha] = task
	return s.saveLocked()
}

func (s *Store) SetTaskBranch(sha string, branch string) error {
	return s.updateTask(sha, func(task *Task) {
		task.Branch = branch
	})
}

func (s *Store) SetTaskMR(sha string, number int, url string) error {
	return s.updateTask(sha, func(task *Task) {
		task.MRNumber = number
		task.MRURL = url
	})
}

func (s *Store) SetTaskMRCommit(sha string, commitSHA string) error {
	return s.updateTask(sha, func(task *Task) {
		task.MRCommitSHA = commitSHA
	})
}

func (s *Store) SetTaskCommunityCommit(sha string, commitSHA string) error {
	return s.updateTask(sha, func(task *Task) {
		task.CommunityCommitSHA = commitSHA
	})
}

func (s *Store) AppendLog(sha string, step string, message string) error {
	return s.updateTask(sha, func(task *Task) {
		task.Logs = append(task.Logs, LogEntry{
			Time:    timestamp(),
			Step:    step,
			Message: message,
		})
	})
}

func (s *Store) RetryTask(sha string) error {
	return s.updateTask(sha, func(task *Task) {
		task.Status = StatusPending
		task.Error = ""
		task.Attempts++
	})
}

func (s *Store) updateTask(sha string, fn func(*Task)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.data.Tasks[sha]
	if !ok {
		return fmt.Errorf("task %s does not exist", sha)
	}
	fn(&task)
	task.UpdatedAt = timestamp()
	s.data.Tasks[sha] = task
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	s.data.UpdatedAt = timestamp()
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
