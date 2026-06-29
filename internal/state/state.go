package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mr-queue/internal/config"
)

const (
	StatusPending                = "pending"
	StatusRunning                = "running"
	StatusPushed                 = "pushed"
	StatusMROpen                 = "mr_open"
	StatusWaitingRequiredComment = "waiting_required_comment"
	StatusWaitingExternalMerge   = "waiting_external_merge"
	StatusReviewed               = "reviewed"
	StatusMerged                 = "merged"
	StatusSkipped                = "skipped"
	StatusFailed                 = "failed"
)

type Store struct {
	path string
	mu   sync.Mutex
	data Snapshot
}

type Snapshot struct {
	Tasks                 map[string]Task          `json:"tasks"`
	TaskRecords           map[string]Task          `json:"task_records,omitempty"`
	ConfigVersions        map[string]ConfigVersion `json:"config_versions,omitempty"`
	TaskScopes            map[string]TaskScope     `json:"task_scopes,omitempty"`
	ActiveConfigVersionID string                   `json:"active_config_version_id,omitempty"`
	ActiveScopeID         string                   `json:"active_scope_id,omitempty"`
	Paused                bool                     `json:"paused"`
	UpdatedAt             string                   `json:"updated_at"`
}

type Task struct {
	SHA                        string     `json:"sha"`
	Subject                    string     `json:"subject"`
	Status                     string     `json:"status"`
	QueueIndex                 int        `json:"queue_index"`
	ConfigVersionID            string     `json:"config_version_id,omitempty"`
	ScopeID                    string     `json:"scope_id,omitempty"`
	SourceRepo                 string     `json:"source_repo,omitempty"`
	SourceBranch               string     `json:"source_branch,omitempty"`
	TargetRepo                 string     `json:"target_repo,omitempty"`
	TargetBranch               string     `json:"target_branch,omitempty"`
	CommitRange                string     `json:"commit_range,omitempty"`
	CherryPickConflictStrategy string     `json:"cherry_pick_conflict_strategy,omitempty"`
	Branch                     string     `json:"branch,omitempty"`
	MRCommitSHA                string     `json:"mr_commit_sha,omitempty"`
	CommunityCommitSHA         string     `json:"community_commit_sha,omitempty"`
	MRNumber                   int        `json:"mr_number,omitempty"`
	MRURL                      string     `json:"mr_url,omitempty"`
	Error                      string     `json:"error,omitempty"`
	Attempts                   int        `json:"attempts"`
	CreatedAt                  string     `json:"created_at"`
	UpdatedAt                  string     `json:"updated_at"`
	Logs                       []LogEntry `json:"logs"`
}

type ConfigVersion struct {
	ID                   string `json:"id"`
	ConfigHash           string `json:"config_hash"`
	Provider             string `json:"provider,omitempty"`
	SourceRepo           string `json:"source_repo,omitempty"`
	SourceBranch         string `json:"source_branch,omitempty"`
	TargetRepo           string `json:"target_repo,omitempty"`
	TargetBranch         string `json:"target_branch,omitempty"`
	RequestedCommitRange string `json:"requested_commit_range,omitempty"`
	ResolvedCommitRange  string `json:"resolved_commit_range,omitempty"`
	MRPushRepo           string `json:"mr_push_repo,omitempty"`
	MRBranchPrefix       string `json:"mr_branch_prefix,omitempty"`
	MRBranchTemplate     string `json:"mr_branch_template,omitempty"`
	MergeMethod          string `json:"merge_method,omitempty"`
	SubmitterTokenEnv    string `json:"submitter_token_env,omitempty"`
	ReviewerTokenEnv     string `json:"reviewer_token_env,omitempty"`
	MaintainerTokenEnv   string `json:"maintainer_token_env,omitempty"`
	SafeConfigJSON       string `json:"safe_config_json,omitempty"`
	ResolvedConfigJSON   string `json:"resolved_config_json,omitempty"`
	Active               bool   `json:"active"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type TaskScope struct {
	ID                   string `json:"id"`
	ConfigVersionID      string `json:"config_version_id"`
	SourceRepo           string `json:"source_repo,omitempty"`
	SourceBranch         string `json:"source_branch,omitempty"`
	TargetRepo           string `json:"target_repo,omitempty"`
	TargetBranch         string `json:"target_branch,omitempty"`
	RequestedCommitRange string `json:"requested_commit_range,omitempty"`
	ResolvedCommitRange  string `json:"resolved_commit_range,omitempty"`
	MRPushRepo           string `json:"mr_push_repo,omitempty"`
	MRBranchTemplate     string `json:"mr_branch_template,omitempty"`
	CommitCount          int    `json:"commit_count"`
	SyncedAt             string `json:"synced_at"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type QueueTask struct {
	SHA     string
	Subject string
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
	store.ensureDatabaseMaps()
	return store, nil
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyData := Snapshot{
		Tasks:                 map[string]Task{},
		TaskRecords:           map[string]Task{},
		ConfigVersions:        map[string]ConfigVersion{},
		TaskScopes:            map[string]TaskScope{},
		ActiveConfigVersionID: s.data.ActiveConfigVersionID,
		ActiveScopeID:         s.data.ActiveScopeID,
		Paused:                s.data.Paused,
		UpdatedAt:             s.data.UpdatedAt,
	}
	for sha, task := range s.data.Tasks {
		copyData.Tasks[sha] = copyTask(task)
	}
	for key, task := range s.data.TaskRecords {
		copyData.TaskRecords[key] = copyTask(task)
	}
	for id, version := range s.data.ConfigVersions {
		copyData.ConfigVersions[id] = version
	}
	for id, scope := range s.data.TaskScopes {
		copyData.TaskScopes[id] = scope
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
	return s.upsertTask(sha, subject, 0, false)
}

func (s *Store) UpsertTaskAt(sha string, subject string, queueIndex int) error {
	return s.upsertTask(sha, subject, queueIndex, true)
}

func (s *Store) ReplaceQueueTasks(tasks []QueueTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatabaseMaps()
	now := timestamp()
	next := map[string]Task{}
	for i, queueTask := range tasks {
		task, ok := s.data.Tasks[queueTask.SHA]
		if !ok {
			task = Task{
				SHA:       queueTask.SHA,
				Status:    StatusPending,
				CreatedAt: now,
			}
		}
		task.Subject = queueTask.Subject
		task.QueueIndex = i
		task.UpdatedAt = now
		next[queueTask.SHA] = task
	}
	s.data.Tasks = next
	return s.saveLocked()
}

func (s *Store) ReplaceQueueTasksForConfig(cfg config.Config, tasks []QueueTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatabaseMaps()
	now := timestamp()
	version := configVersionFromConfig(cfg, now)
	if existing, ok := s.data.ConfigVersions[version.ID]; ok && existing.CreatedAt != "" {
		version.CreatedAt = existing.CreatedAt
	}
	for id, existing := range s.data.ConfigVersions {
		existing.Active = false
		existing.UpdatedAt = now
		s.data.ConfigVersions[id] = existing
	}
	version.Active = true
	version.UpdatedAt = now
	s.data.ConfigVersions[version.ID] = version

	scope := taskScopeFromConfig(cfg, version, len(tasks), now)
	if existing, ok := s.data.TaskScopes[scope.ID]; ok && existing.CreatedAt != "" {
		scope.CreatedAt = existing.CreatedAt
	}
	s.data.TaskScopes[scope.ID] = scope
	s.data.ActiveConfigVersionID = version.ID
	s.data.ActiveScopeID = scope.ID

	next := map[string]Task{}
	for i, queueTask := range tasks {
		recordKey := taskRecordKey(scope.ID, queueTask.SHA)
		task, ok := s.data.TaskRecords[recordKey]
		if !ok {
			if existing, exists := s.data.Tasks[queueTask.SHA]; exists && (existing.ScopeID == "" || existing.ScopeID == scope.ID) {
				task = existing
			} else {
				task = Task{
					SHA:       queueTask.SHA,
					Status:    StatusPending,
					CreatedAt: now,
				}
			}
		}
		task.Subject = queueTask.Subject
		task.QueueIndex = i
		task.ConfigVersionID = version.ID
		task.ScopeID = scope.ID
		task.SourceRepo = version.SourceRepo
		task.SourceBranch = version.SourceBranch
		task.TargetRepo = version.TargetRepo
		task.TargetBranch = version.TargetBranch
		task.CommitRange = version.ResolvedCommitRange
		task.UpdatedAt = now
		next[queueTask.SHA] = task
		s.data.TaskRecords[recordKey] = task
	}
	s.data.Tasks = next
	return s.saveLocked()
}

func (s *Store) SelectScope(scopeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatabaseMaps()
	scope, ok := s.data.TaskScopes[scopeID]
	if !ok {
		return fmt.Errorf("task scope %s does not exist", scopeID)
	}
	if _, ok := s.data.ConfigVersions[scope.ConfigVersionID]; !ok {
		return fmt.Errorf("config version %s does not exist", scope.ConfigVersionID)
	}
	next := map[string]Task{}
	for key, task := range s.data.TaskRecords {
		if task.ScopeID != scopeID && !strings.HasPrefix(key, scopeID+":") {
			continue
		}
		next[task.SHA] = task
	}
	s.data.Tasks = next
	s.data.ActiveScopeID = scopeID
	s.data.ActiveConfigVersionID = scope.ConfigVersionID
	now := timestamp()
	for id, version := range s.data.ConfigVersions {
		version.Active = id == scope.ConfigVersionID
		version.UpdatedAt = now
		s.data.ConfigVersions[id] = version
	}
	return s.saveLocked()
}

func (s *Store) upsertTask(sha string, subject string, queueIndex int, setIndex bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatabaseMaps()
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
	if setIndex {
		task.QueueIndex = queueIndex
	}
	task.UpdatedAt = now
	s.data.Tasks[sha] = task
	s.upsertTaskRecordLocked(task)
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
	s.upsertTaskRecordLocked(task)
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
	return s.RetryTaskWithConflictStrategy(sha, "")
}

func (s *Store) RetryTaskWithConflictStrategy(sha string, strategy string) error {
	return s.updateTask(sha, func(task *Task) {
		task.Status = StatusPending
		task.Error = ""
		task.Attempts++
		task.CherryPickConflictStrategy = strategy
	})
}

func (s *Store) updateTask(sha string, fn func(*Task)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatabaseMaps()
	task, ok := s.data.Tasks[sha]
	if !ok {
		return fmt.Errorf("task %s does not exist", sha)
	}
	fn(&task)
	task.UpdatedAt = timestamp()
	s.data.Tasks[sha] = task
	s.upsertTaskRecordLocked(task)
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

func (s *Store) ensureDatabaseMaps() {
	if s.data.Tasks == nil {
		s.data.Tasks = map[string]Task{}
	}
	if s.data.TaskRecords == nil {
		s.data.TaskRecords = map[string]Task{}
	}
	if s.data.ConfigVersions == nil {
		s.data.ConfigVersions = map[string]ConfigVersion{}
	}
	if s.data.TaskScopes == nil {
		s.data.TaskScopes = map[string]TaskScope{}
	}
}

func (s *Store) upsertTaskRecordLocked(task Task) {
	if task.ScopeID == "" || task.SHA == "" {
		return
	}
	s.data.TaskRecords[taskRecordKey(task.ScopeID, task.SHA)] = task
}

func copyTask(task Task) Task {
	task.Logs = append([]LogEntry(nil), task.Logs...)
	return task
}

func configVersionFromConfig(cfg config.Config, now string) ConfigVersion {
	safeConfig := cfg.Safe()
	version := ConfigVersion{
		Provider:             cfg.Provider,
		SourceRepo:           sourceRepoText(cfg),
		SourceBranch:         cfg.Source.Branch,
		TargetRepo:           targetRepoText(cfg),
		TargetBranch:         targetBranchText(cfg),
		RequestedCommitRange: requestedCommitRange(cfg),
		ResolvedCommitRange:  cfg.Workflow.CommitRange,
		MRPushRepo:           mrPushRepoText(cfg),
		MRBranchPrefix:       cfg.Private.BranchPrefix,
		MRBranchTemplate:     cfg.Private.BranchTemplate,
		MergeMethod:          cfg.Workflow.MergeMethod,
		SubmitterTokenEnv:    cfg.Auth.Submitter.TokenEnv,
		ReviewerTokenEnv:     cfg.Auth.Reviewer.TokenEnv,
		MaintainerTokenEnv:   cfg.Auth.Maintainer.TokenEnv,
		SafeConfigJSON:       safeConfig,
		ResolvedConfigJSON:   safeConfig,
		Active:               true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	version.ConfigHash = hashID("config", map[string]string{
		"provider":               version.Provider,
		"source_repo":            version.SourceRepo,
		"source_branch":          version.SourceBranch,
		"target_repo":            version.TargetRepo,
		"target_branch":          version.TargetBranch,
		"requested_commit_range": version.RequestedCommitRange,
		"resolved_commit_range":  version.ResolvedCommitRange,
		"mr_push_repo":           version.MRPushRepo,
		"mr_branch_prefix":       version.MRBranchPrefix,
		"mr_branch_template":     version.MRBranchTemplate,
		"merge_method":           version.MergeMethod,
	})
	version.ID = "cfg-" + version.ConfigHash[:16]
	return version
}

func taskScopeFromConfig(cfg config.Config, version ConfigVersion, commitCount int, now string) TaskScope {
	hash := hashID("scope", map[string]string{
		"config_version_id":     version.ID,
		"resolved_commit_range": version.ResolvedCommitRange,
		"source_repo":           version.SourceRepo,
		"source_branch":         version.SourceBranch,
		"target_repo":           version.TargetRepo,
		"target_branch":         version.TargetBranch,
		"mr_branch_template":    version.MRBranchTemplate,
		"private_remote":        cfg.Private.Remote,
		"community_remote":      cfg.Community.Remote,
		"queue_remote":          cfg.Queue.Remote,
	})
	return TaskScope{
		ID:                   "scope-" + hash[:16],
		ConfigVersionID:      version.ID,
		SourceRepo:           version.SourceRepo,
		SourceBranch:         version.SourceBranch,
		TargetRepo:           version.TargetRepo,
		TargetBranch:         version.TargetBranch,
		RequestedCommitRange: version.RequestedCommitRange,
		ResolvedCommitRange:  version.ResolvedCommitRange,
		MRPushRepo:           version.MRPushRepo,
		MRBranchTemplate:     version.MRBranchTemplate,
		CommitCount:          commitCount,
		SyncedAt:             now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func hashID(kind string, value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(append([]byte(kind+":"), body...))
	return fmt.Sprintf("%x", sum)
}

func taskRecordKey(scopeID string, sha string) string {
	return scopeID + ":" + sha
}

func sourceRepoText(cfg config.Config) string {
	if cfg.Source.Repo != "" {
		return cfg.Source.Repo
	}
	return cfg.Queue.RemoteURL
}

func targetRepoText(cfg config.Config) string {
	if cfg.Target.Repo != "" {
		return cfg.Target.Repo
	}
	if cfg.Community.RemoteURL != "" {
		return cfg.Community.RemoteURL
	}
	if cfg.Community.Owner != "" && cfg.Community.Repo != "" {
		return cfg.Community.Owner + "/" + cfg.Community.Repo
	}
	return ""
}

func targetBranchText(cfg config.Config) string {
	if cfg.Target.Branch != "" {
		return cfg.Target.Branch
	}
	return cfg.Community.Branch
}

func requestedCommitRange(cfg config.Config) string {
	if cfg.Source.Range != "" {
		return cfg.Source.Range
	}
	if cfg.Source.StartSHA != "" || cfg.Source.EndSHA != "" {
		return cfg.Source.StartSHA + "^.." + cfg.Source.EndSHA
	}
	if cfg.Queue.StartSHA != "" || cfg.Queue.EndSHA != "" {
		return cfg.Queue.StartSHA + "^.." + cfg.Queue.EndSHA
	}
	return ""
}

func mrPushRepoText(cfg config.Config) string {
	if cfg.Private.RemoteURL != "" {
		return cfg.Private.RemoteURL
	}
	if cfg.Source.Repo != "" {
		return cfg.Source.Repo
	}
	return cfg.Queue.RemoteURL
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
