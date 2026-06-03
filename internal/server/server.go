package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mr-queue/internal/app"
)

type Server struct {
	runtime *app.Runtime
	mu      sync.Mutex
	running bool
	mode    string
	lastErr string
	cancel  context.CancelFunc
}

func New(runtime *app.Runtime) *Server {
	return &Server{runtime: runtime}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/status", s.status)
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
	s.mu.Unlock()
	respondJSON(w, map[string]interface{}{
		"state":   s.runtime.State.Snapshot(),
		"config":  json.RawMessage(s.runtime.Config.Safe()),
		"running": running,
		"mode":    mode,
		"lastErr": lastErr,
	})
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
	s.cancel = cancel
	s.mu.Unlock()

	go func() {
		err := s.runtime.Runner.RunLoop(ctx, 1200*time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.mode = ""
		s.cancel = nil
		if err != nil && err != context.Canceled {
			s.lastErr = err.Error()
		}
	}()
	respondJSON(w, map[string]string{"status": "started"})
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

func respondJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}
