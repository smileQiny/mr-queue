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
	"mr-queue/internal/runner"
)

type Server struct {
	runtime *app.Runtime
	mu      sync.Mutex
	running bool
	mode    string
	lastErr string
	lastMsg string
	cancel  context.CancelFunc
}

func New(runtime *app.Runtime) *Server {
	s := &Server{runtime: runtime}
	go s.refreshWaitingLoop()
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
	s.mu.Unlock()
	respondJSON(w, map[string]interface{}{
		"state":   s.runtime.State.Snapshot(),
		"config":  json.RawMessage(s.runtime.Config.Safe()),
		"running": running,
		"mode":    mode,
		"lastErr": lastErr,
		"lastMsg": lastMsg,
	})
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
	s.mu.Unlock()

	go func() {
		count, err := s.runtime.Runner.SyncQueue()
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
	s.mu.Unlock()

	go func() {
		err := s.runtime.Runner.RunOnce()
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
		result, err := s.runtime.Runner.RunLoopWithOptions(ctx, options)
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
		minDelay, maxDelay, err := s.runtime.Config.Workflow.WaitCheckDelayRange()
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
		s.mu.Unlock()

		err = s.runtime.Runner.RefreshWaiting()

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
