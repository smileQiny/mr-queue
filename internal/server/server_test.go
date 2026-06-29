package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"mr-queue/internal/app"
	"mr-queue/internal/config"
	"mr-queue/internal/doctor"
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

func TestStatusIncludesStartupDoctorReport(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := doctor.Report{OK: true, Checks: []doctor.Check{{Name: "config", Status: doctor.StatusOK, Message: "loaded"}}}
	s := &Server{
		runtime:      &app.Runtime{Config: &config.Config{}, State: store},
		doctorReport: &report,
	}
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	s.status(w, req)

	var body struct {
		Doctor *doctor.Report `json:"doctor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if body.Doctor == nil || !body.Doctor.OK || body.Doctor.Checks[0].Name != "config" {
		t.Fatalf("doctor = %#v", body.Doctor)
	}
}

func TestDoctorAPIRunsDoctorAndReturnsReport(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		runtime: &app.Runtime{Config: &config.Config{}, State: store},
		doctorRunner: fakeDoctorRunner{report: doctor.Report{
			OK:     true,
			Checks: []doctor.Check{{Name: "workspace", Status: doctor.StatusOK, Message: "ready"}},
		}},
	}
	req := httptest.NewRequest("POST", "/api/doctor", nil)
	w := httptest.NewRecorder()

	s.doctor(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var report doctor.Report
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode doctor report: %v", err)
	}
	if !report.OK || report.Checks[0].Name != "workspace" {
		t.Fatalf("report = %#v", report)
	}
}

func TestSelectScopeAPISwitchesActiveTasks(t *testing.T) {
	t.Setenv("GITCODE_SUBMITTER_TOKEN", "submit-token")
	t.Setenv("GITCODE_REVIEWER_TOKEN", "review-token")
	store, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	first := config.Config{
		Provider: "atomgit",
		Source:   config.Source{Repo: "atomgit.com/source/project", Branch: "feature-a"},
		Target:   config.Target{Repo: "atomgit.com/target/project", Branch: "master"},
		Workflow: config.Workflow{CommitRange: "target/master..source/feature-a", MergeMethod: "external"},
		Private:  config.Private{RemoteURL: "https://atomgit.com/source/project.git", BranchPrefix: "feat", BranchTemplate: "{prefix}-{sha12}"},
		Auth: config.Auth{
			Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN"},
			Reviewer:  config.Credential{TokenEnv: "GITCODE_REVIEWER_TOKEN"},
		},
	}
	second := first
	second.Source.Branch = "feature-b"
	second.Workflow.CommitRange = "target/master..source/feature-b"
	if err := store.ReplaceQueueTasksForConfig(first, []state.QueueTask{{SHA: "aaa111", Subject: "First"}}); err != nil {
		t.Fatal(err)
	}
	firstScope := store.Snapshot().ActiveScopeID
	if err := store.ReplaceQueueTasksForConfig(second, []state.QueueTask{{SHA: "bbb222", Subject: "Second"}}); err != nil {
		t.Fatal(err)
	}
	s := &Server{runtime: &app.Runtime{Config: &config.Config{}, State: store}}
	req := httptest.NewRequest("POST", "/api/select-scope?scope_id="+firstScope, nil)
	w := httptest.NewRecorder()

	s.selectScope(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	snapshot := store.Snapshot()
	if snapshot.ActiveScopeID != firstScope {
		t.Fatalf("active scope = %q, want %q", snapshot.ActiveScopeID, firstScope)
	}
	if _, ok := snapshot.Tasks["aaa111"]; !ok {
		t.Fatalf("first scope task missing: %#v", snapshot.Tasks)
	}
	if _, ok := snapshot.Tasks["bbb222"]; ok {
		t.Fatalf("second scope task still active: %#v", snapshot.Tasks)
	}
	if s.runtime.Config.Source.Branch != "feature-a" {
		t.Fatalf("runtime source branch = %q, want feature-a", s.runtime.Config.Source.Branch)
	}
	if s.runtime.Runner == nil {
		t.Fatal("runtime runner was not rebound for selected scope")
	}
}

type fakeDoctorRunner struct {
	report doctor.Report
}

func (r fakeDoctorRunner) Run(fix bool) doctor.Report {
	return r.report
}
