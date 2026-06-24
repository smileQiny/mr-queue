package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"mr-queue/internal/app"
	"mr-queue/internal/config"
	"mr-queue/internal/state"
)

func TestLoopOptionsFromRequestOverridesConfigDefaults(t *testing.T) {
	s := &Server{runtime: &app.Runtime{
		Config: &config.Config{
			Workflow: config.Workflow{
				WaitCheckDelayMin: "10s",
				WaitCheckDelayMax: "30s",
				NextPRDelayMin:    "1m",
				NextPRDelayMax:    "5m",
			},
		},
	}}
	req := httptest.NewRequest("POST", "/api/run-loop?wait_check_delay_min=2s&wait_check_delay_max=4s&next_pr_delay_min=1m&next_pr_delay_max=5m&max_merged_commits=3&work_window_start=08:00&work_window_end=23:30", nil)

	options, err := s.loopOptionsFromRequest(req)
	if err != nil {
		t.Fatalf("loopOptionsFromRequest returned error: %v", err)
	}

	if options.WaitCheckDelayMin != 2*time.Second || options.WaitCheckDelayMax != 4*time.Second {
		t.Fatalf("wait check delay range = %s..%s", options.WaitCheckDelayMin, options.WaitCheckDelayMax)
	}
	if options.NextPRDelayMin != time.Minute || options.NextPRDelayMax != 5*time.Minute {
		t.Fatalf("next PR delay range = %s..%s", options.NextPRDelayMin, options.NextPRDelayMax)
	}
	if options.MaxMergedCommits != 3 {
		t.Fatalf("max merged commits = %d", options.MaxMergedCommits)
	}
	if options.WorkWindowStart != "08:00" || options.WorkWindowEnd != "23:30" {
		t.Fatalf("work window = %q..%q", options.WorkWindowStart, options.WorkWindowEnd)
	}
}

func TestStatusIncludesLastMessage(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		runtime: &app.Runtime{Config: &config.Config{}, State: store},
		lastMsg: "auto run stopped: reached max merged commits: 3/3",
	}
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	s.status(w, req)

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if body["lastMsg"] != "auto run stopped: reached max merged commits: 3/3" {
		t.Fatalf("lastMsg = %#v", body["lastMsg"])
	}
}
