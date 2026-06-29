package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"mr-queue/internal/app"
	"mr-queue/internal/config"
	"mr-queue/internal/doctor"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/runner"
)

type DoctorRunner interface {
	Run(fix bool) doctor.Report
}

type Server struct {
	runtime      *app.Runtime
	doctorRunner DoctorRunner
	doctorReport *doctor.Report
	mu           sync.Mutex
	running      bool
	mode         string
	lastErr      string
	lastMsg      string
	cancel       context.CancelFunc
}

func New(runtime *app.Runtime) *Server {
	s := &Server{
		runtime:      runtime,
		doctorRunner: runtimeDoctorRunner{runtime: runtime},
	}
	go s.refreshWaitingLoop()
	go s.runStartupDoctor()
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/status", s.status)
	mux.HandleFunc("/api/sync-queue", s.syncQueue)
	mux.HandleFunc("/api/run-once", s.runOnce)
	mux.HandleFunc("/api/run-loop", s.runLoop)
	mux.HandleFunc("/api/stop", s.stop)
	mux.HandleFunc("/api/pause", s.pause)
	mux.HandleFunc("/api/resume", s.resume)
	mux.HandleFunc("/api/retry", s.retry)
	mux.HandleFunc("/api/select-scope", s.selectScope)
	mux.HandleFunc("/api/doctor", s.doctor)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	running := s.running
	mode := s.mode
	lastErr := s.lastErr
	lastMsg := s.lastMsg
	doctorReport := s.doctorReport
	snapshot := s.runtime.State.Snapshot()
	configSafe := s.runtime.Config.Safe()
	s.mu.Unlock()
	respondJSON(w, map[string]interface{}{
		"state":   snapshot,
		"config":  json.RawMessage(configSafe),
		"running": running,
		"mode":    mode,
		"lastErr": lastErr,
		"lastMsg": lastMsg,
		"doctor":  doctorReport,
	})
}

func (s *Server) runStartupDoctor() {
	if s.doctorRunner == nil {
		return
	}
	report := s.doctorRunner.Run(false)
	s.mu.Lock()
	s.doctorReport = &report
	s.mu.Unlock()
}

func (s *Server) doctor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fix := r.URL.Query().Get("fix") == "true" || r.URL.Query().Get("fix") == "1"
	runner := s.doctorRunner
	if runner == nil {
		runner = runtimeDoctorRunner{runtime: s.runtime}
	}
	report := runner.Run(fix)
	s.mu.Lock()
	s.doctorReport = &report
	s.mu.Unlock()
	respondJSON(w, report)
}

type runtimeDoctorRunner struct {
	runtime *app.Runtime
}

func (r runtimeDoctorRunner) Run(fix bool) doctor.Report {
	cfg := *r.runtime.Config
	return doctor.Run(
		cfg,
		doctor.Options{Fix: fix},
		doctor.LocalGitChecker{Dir: cfg.Local.Path, Username: cfg.Private.HeadNamespace, AccessToken: cfg.Auth.Submitter.Token},
		gitcode.NewClientForProvider(cfg.Provider, reviewerTokenForDoctor(cfg)),
	)
}

func reviewerTokenForDoctor(cfg config.Config) string {
	if cfg.Auth.Reviewer.Token != "" {
		return cfg.Auth.Reviewer.Token
	}
	return cfg.Auth.Maintainer.Token
}

func (s *Server) syncQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		respondJSON(w, map[string]string{"status": "already-running"})
		return
	}
	s.running = true
	s.mode = "sync"
	s.lastErr = ""
	s.lastMsg = ""
	runnerRef := s.runtime.Runner
	s.mu.Unlock()

	go func() {
		count, err := runnerRef.SyncQueue()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.mode = ""
		if err != nil {
			s.lastErr = err.Error()
			return
		}
		s.lastMsg = fmt.Sprintf("synced %d queue commits", count)
	}()
	respondJSON(w, map[string]string{"status": "started"})
}

func (s *Server) selectScope(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scopeID := r.URL.Query().Get("scope_id")
	if scopeID == "" {
		http.Error(w, "scope_id is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		http.Error(w, "cannot switch task while running", http.StatusConflict)
		return
	}
	snapshot := s.runtime.State.Snapshot()
	scope, ok := snapshot.TaskScopes[scopeID]
	if !ok {
		http.Error(w, "task scope "+scopeID+" does not exist", http.StatusBadRequest)
		return
	}
	version, ok := snapshot.ConfigVersions[scope.ConfigVersionID]
	if !ok {
		http.Error(w, "config version "+scope.ConfigVersionID+" does not exist", http.StatusBadRequest)
		return
	}
	configJSON := version.ResolvedConfigJSON
	if configJSON == "" {
		configJSON = version.SafeConfigJSON
	}
	if configJSON == "" {
		http.Error(w, "config version "+scope.ConfigVersionID+" has no stored config", http.StatusBadRequest)
		return
	}
	cfg, err := config.LoadSafeJSON([]byte(configJSON))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.runtime.RebindConfig(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.runtime.State.SelectScope(scopeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, map[string]string{"status": "selected", "scope_id": scopeID})
}

func (s *Server) runOnce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		respondJSON(w, map[string]string{"status": "already-running"})
		return
	}
	s.running = true
	s.mode = "once"
	s.lastErr = ""
	s.lastMsg = ""
	runnerRef := s.runtime.Runner
	s.mu.Unlock()

	go func() {
		err := runnerRef.RunOnce()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.mode = ""
		s.cancel = nil
		if err != nil {
			s.lastErr = err.Error()
		}
	}()
	respondJSON(w, map[string]string{"status": "started"})
}

func (s *Server) runLoop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		respondJSON(w, map[string]string{"status": "already-running"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.mode = "loop"
	s.lastErr = ""
	s.lastMsg = ""
	s.cancel = cancel
	runnerRef := s.runtime.Runner
	s.mu.Unlock()

	options, err := s.loopOptionsFromRequest(r)
	if err != nil {
		cancel()
		s.mu.Lock()
		s.running = false
		s.mode = ""
		s.cancel = nil
		s.lastErr = err.Error()
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	options.OnProgress = func(progress runner.LoopProgress) {
		s.mu.Lock()
		s.lastMsg = "auto run: " + progress.Message
		s.mu.Unlock()
	}

	go func() {
		result, err := runnerRef.RunLoopWithOptions(ctx, options)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.mode = ""
		s.cancel = nil
		if err != nil && err != context.Canceled {
			s.lastErr = err.Error()
			return
		}
		if result.StopReason != "" {
			s.lastMsg = "auto run stopped: " + result.StopReason
		}
	}()
	respondJSON(w, map[string]string{"status": "started"})
}

func (s *Server) loopOptionsFromRequest(r *http.Request) (runner.LoopOptions, error) {
	waitMinDelay, waitMaxDelay, err := s.runtime.Config.Workflow.WaitCheckDelayRange()
	if err != nil {
		return runner.LoopOptions{}, err
	}
	nextMinDelay, nextMaxDelay, err := s.runtime.Config.Workflow.NextPRDelayRange()
	if err != nil {
		return runner.LoopOptions{}, err
	}
	if value := firstFormValue(r, "wait_check_delay_min", "loop_delay_min"); value != "" {
		waitMinDelay, err = time.ParseDuration(value)
		if err != nil {
			return runner.LoopOptions{}, fmt.Errorf("parse wait_check_delay_min: %w", err)
		}
	}
	if value := firstFormValue(r, "wait_check_delay_max", "loop_delay_max"); value != "" {
		waitMaxDelay, err = time.ParseDuration(value)
		if err != nil {
			return runner.LoopOptions{}, fmt.Errorf("parse wait_check_delay_max: %w", err)
		}
	}
	if value := r.FormValue("next_pr_delay_min"); value != "" {
		nextMinDelay, err = time.ParseDuration(value)
		if err != nil {
			return runner.LoopOptions{}, fmt.Errorf("parse next_pr_delay_min: %w", err)
		}
	}
	if value := r.FormValue("next_pr_delay_max"); value != "" {
		nextMaxDelay, err = time.ParseDuration(value)
		if err != nil {
			return runner.LoopOptions{}, fmt.Errorf("parse next_pr_delay_max: %w", err)
		}
	}
	if waitMinDelay <= 0 || waitMaxDelay <= 0 || nextMinDelay <= 0 || nextMaxDelay <= 0 {
		return runner.LoopOptions{}, fmt.Errorf("loop delays must be positive")
	}
	if waitMaxDelay < waitMinDelay {
		return runner.LoopOptions{}, fmt.Errorf("wait_check_delay_max must be >= wait_check_delay_min")
	}
	if nextMaxDelay < nextMinDelay {
		return runner.LoopOptions{}, fmt.Errorf("next_pr_delay_max must be >= next_pr_delay_min")
	}
	maxMergedCommits := 0
	if value := r.FormValue("max_merged_commits"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return runner.LoopOptions{}, fmt.Errorf("parse max_merged_commits: %w", err)
		}
		if parsed < 0 {
			return runner.LoopOptions{}, fmt.Errorf("max_merged_commits must be >= 0")
		}
		maxMergedCommits = parsed
	}
	return runner.LoopOptions{
		WaitCheckDelayMin: waitMinDelay,
		WaitCheckDelayMax: waitMaxDelay,
		NextPRDelayMin:    nextMinDelay,
		NextPRDelayMax:    nextMaxDelay,
		MaxMergedCommits:  maxMergedCommits,
		WorkWindowStart:   r.FormValue("work_window_start"),
		WorkWindowEnd:     r.FormValue("work_window_end"),
	}, nil
}

func firstFormValue(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := r.FormValue(name); value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	respondJSON(w, map[string]string{"status": "stopping"})
}

func (s *Server) pause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.runtime.State.SetPaused(true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "paused"})
}

func (s *Server) resume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.runtime.State.SetPaused(false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "resumed"})
}

func (s *Server) retry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sha := r.URL.Query().Get("sha")
	if sha == "" {
		http.Error(w, "sha is required", http.StatusBadRequest)
		return
	}
	if err := s.runtime.State.RetryTask(sha); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "pending"})
}

func (s *Server) refreshWaitingLoop() {
	for {
		s.mu.Lock()
		cfg := s.runtime.Config
		s.mu.Unlock()
		minDelay, maxDelay, err := cfg.Workflow.WaitCheckDelayRange()
		delay := minDelay
		if err != nil {
			delay = time.Minute
		} else if maxDelay > minDelay {
			delay = minDelay + time.Duration(time.Now().UnixNano()%int64(maxDelay-minDelay+1))
		}
		time.Sleep(delay)
		s.mu.Lock()
		if s.running {
			s.mu.Unlock()
			continue
		}
		s.running = true
		s.mode = "refresh-waiting"
		runnerRef := s.runtime.Runner
		s.mu.Unlock()

		err = runnerRef.RefreshWaiting()

		s.mu.Lock()
		s.running = false
		s.mode = ""
		if err != nil {
			s.lastErr = err.Error()
		} else if s.lastErr != "" {
			s.lastErr = ""
		}
		s.mu.Unlock()
	}
}

func respondJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}
