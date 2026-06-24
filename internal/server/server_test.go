package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"mr-queue/internal/app"
	"mr-queue/internal/config"
)

func TestLoopOptionsFromRequestOverridesConfigDefaults(t *testing.T) {
	s := &Server{runtime: &app.Runtime{
		Config: &config.Config{
			Workflow: config.Workflow{
				LoopDelayMin: "1m",
				LoopDelayMax: "5m",
			},
		},
	}}
	req := httptest.NewRequest("POST", "/api/run-loop?loop_delay_min=2s&loop_delay_max=4s&max_merged_commits=3&work_window_start=08:00&work_window_end=23:30", nil)

	options, err := s.loopOptionsFromRequest(req)
	if err != nil {
		t.Fatalf("loopOptionsFromRequest returned error: %v", err)
	}

	if options.MinDelay != 2*time.Second || options.MaxDelay != 4*time.Second {
		t.Fatalf("delay range = %s..%s", options.MinDelay, options.MaxDelay)
	}
	if options.MaxMergedCommits != 3 {
		t.Fatalf("max merged commits = %d", options.MaxMergedCommits)
	}
	if options.WorkWindowStart != "08:00" || options.WorkWindowEnd != "23:30" {
		t.Fatalf("work window = %q..%q", options.WorkWindowStart, options.WorkWindowEnd)
	}
}
