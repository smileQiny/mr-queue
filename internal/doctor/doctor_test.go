package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mr-queue/internal/config"
)

func TestRunReportsConfigWorkspaceAndTokenChecks(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Provider:  "gitcode",
		Workspace: repo,
		Local:     config.Local{Path: repo},
		Queue: config.Queue{
			Remote:    "mrq-source",
			RemoteURL: "https://gitcode.com/source/project.git",
			Branch:    "queue",
			BaseRef:   "mrq-target/master",
		},
		Private: config.Private{
			Remote:        "mrq-source",
			RemoteURL:     "https://gitcode.com/source/project.git",
			HeadNamespace: "source",
		},
		Community: config.Community{
			Remote:    "mrq-target",
			RemoteURL: "https://gitcode.com/target/project.git",
			Owner:     "target",
			Repo:      "project",
			Branch:    "master",
		},
		Workflow: config.Workflow{CommitRange: "mrq-target/master..mrq-source/queue", MergeMethod: "external"},
		Auth: config.Auth{
			Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "submit-token"},
			Reviewer:  config.Credential{TokenEnv: "GITCODE_REVIEWER_TOKEN", Token: "review-token"},
		},
	}

	report := Run(cfg, Options{}, &fakeGit{isRepo: true}, fakeAPI{ok: true})

	assertCheck(t, report, "config", StatusOK)
	assertCheck(t, report, "workspace", StatusOK)
	assertCheck(t, report, "auth.submitter", StatusOK)
	assertCheck(t, report, "auth.reviewer", StatusOK)
	assertCheck(t, report, "auth.maintainer", StatusOK)
	assertCheck(t, report, "git.source.fetch", StatusOK)
	assertCheck(t, report, "git.target.fetch", StatusOK)
	assertCheck(t, report, "git.source.push_auth", StatusOK)
	assertCheck(t, report, "git.commit_range", StatusOK)
	assertCheck(t, report, "api.target", StatusOK)
	if !report.OK {
		t.Fatalf("report should be OK: %#v", report)
	}
}

func TestRunReportsWorkspaceNotGitRepository(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Local:     config.Local{Path: dir},
		Workspace: dir,
		Auth:      config.Auth{Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "submit-token"}},
	}

	report := Run(cfg, Options{}, &fakeGit{isRepo: false}, fakeAPI{})

	check := findCheck(report, "workspace")
	if check.Status != StatusError {
		t.Fatalf("workspace status = %q", check.Status)
	}
	if !strings.Contains(check.Message, "not a git repository") {
		t.Fatalf("workspace message = %q", check.Message)
	}
	if report.OK {
		t.Fatal("report should not be OK")
	}
	if check := findCheck(report, "git.source.fetch"); check.Name != "" {
		t.Fatalf("fetch should be skipped when workspace is invalid: %#v", check)
	}
}

func TestRunReportsRemotePlanWithoutFix(t *testing.T) {
	cfg := config.Config{
		Local: config.Local{Path: "/repo"},
		Queue: config.Queue{
			Remote:    "mrq-source",
			RemoteURL: "https://gitcode.com/source/project.git",
		},
		Community: config.Community{
			Remote:    "mrq-target",
			RemoteURL: "https://gitcode.com/target/project.git",
		},
		Auth: config.Auth{Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "submit-token"}},
	}
	git := &fakeGit{
		isRepo: true,
		remoteStates: map[string]RemoteState{
			"mrq-source": {Name: "mrq-source", URL: "", DesiredURL: "https://gitcode.com/source/project.git", Exists: false},
			"mrq-target": {Name: "mrq-target", URL: "", DesiredURL: "https://gitcode.com/target/project.git", Exists: false},
		},
	}

	report := Run(cfg, Options{Fix: false}, git, fakeAPI{})

	assertCheck(t, report, "git.remote.mrq-source", StatusWarn)
	assertCheck(t, report, "git.remote.mrq-target", StatusWarn)
	assertCheck(t, report, "git.source.fetch", StatusWarn)
	assertCheck(t, report, "git.target.fetch", StatusWarn)
	if len(git.fixedRemotes) != 0 {
		t.Fatalf("unexpected fixed remotes: %#v", git.fixedRemotes)
	}
}

func TestRunFixesManagedRemotesWhenRequested(t *testing.T) {
	cfg := config.Config{
		Local: config.Local{Path: "/repo"},
		Queue: config.Queue{
			Remote:    "mrq-source",
			RemoteURL: "https://gitcode.com/source/project.git",
		},
		Auth: config.Auth{Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "submit-token"}},
	}
	git := &fakeGit{
		isRepo: true,
		remoteStates: map[string]RemoteState{
			"mrq-source": {Name: "mrq-source", URL: "", DesiredURL: "https://gitcode.com/source/project.git", Exists: false},
		},
	}

	report := Run(cfg, Options{Fix: true}, git, fakeAPI{})

	assertCheck(t, report, "git.remote.mrq-source", StatusOK)
	if len(git.fixedRemotes) != 1 || git.fixedRemotes[0] != "mrq-source" {
		t.Fatalf("fixed remotes = %#v", git.fixedRemotes)
	}
}

func TestRunReportsSourcePushAuthFailure(t *testing.T) {
	cfg := config.Config{
		Local: config.Local{Path: "/repo"},
		Queue: config.Queue{
			Remote:    "mrq-source",
			RemoteURL: "https://gitcode.com/source/project.git",
		},
		Private: config.Private{
			Remote:       "mrq-source",
			RemoteURL:    "https://gitcode.com/source/project.git",
			BranchPrefix: "feat",
		},
		Auth: config.Auth{
			Submitter: config.Credential{TokenEnv: "GITCODE_SUBMITTER_TOKEN", Token: "submit-token"},
		},
	}
	git := &fakeGit{isRepo: true, pushErr: errFake("permission denied")}

	report := Run(cfg, Options{}, git, fakeAPI{})

	check := findCheck(report, "git.source.push_auth")
	if check.Status != StatusError {
		t.Fatalf("source push auth status = %q, want %q", check.Status, StatusError)
	}
	if !strings.Contains(check.Message, "cannot dry-run push") {
		t.Fatalf("source push auth message = %q", check.Message)
	}
	if !strings.Contains(check.Fix, "GITCODE_SUBMITTER_TOKEN") {
		t.Fatalf("source push auth fix should mention submitter token: %q", check.Fix)
	}
	if !strings.Contains(check.Fix, "SSH key") {
		t.Fatalf("source push auth fix should mention SSH key: %q", check.Fix)
	}
	if report.OK {
		t.Fatal("report should not be OK")
	}
	if len(git.pushChecks) != 1 || git.pushChecks[0] != "mrq-source:feat-doctor-check" {
		t.Fatalf("push checks = %#v", git.pushChecks)
	}
}

type fakeGit struct {
	isRepo       bool
	remoteStates map[string]RemoteState
	fixedRemotes []string
	fetchErrs    map[string]error
	rangeErr     error
	pushErr      error
	pushChecks   []string
}

func (g fakeGit) IsRepository(path string) bool {
	return g.isRepo
}

func (g fakeGit) RemoteState(name string, desiredURL string) (RemoteState, error) {
	if state, ok := g.remoteStates[name]; ok {
		return state, nil
	}
	return RemoteState{Name: name, URL: desiredURL, DesiredURL: desiredURL, Exists: true}, nil
}

func (g *fakeGit) EnsureRemote(name string, desiredURL string) error {
	g.fixedRemotes = append(g.fixedRemotes, name)
	return nil
}

func (g fakeGit) Fetch(remote string) error {
	if g.fetchErrs == nil {
		return nil
	}
	return g.fetchErrs[remote]
}

func (g fakeGit) CheckCommitRange(commitRange string) error {
	return g.rangeErr
}

func (g *fakeGit) CheckPushAccess(remote string, branch string) error {
	g.pushChecks = append(g.pushChecks, remote+":"+branch)
	return g.pushErr
}

type errFake string

func (e errFake) Error() string {
	return string(e)
}

type fakeAPI struct {
	ok  bool
	err error
}

func (a fakeAPI) CheckRepository(owner string, repo string, token string) error {
	if a.err != nil {
		return a.err
	}
	return nil
}

func assertCheck(t *testing.T, report Report, name string, status Status) {
	t.Helper()
	check := findCheck(report, name)
	if check.Name == "" {
		t.Fatalf("missing check %q in %#v", name, report.Checks)
	}
	if check.Status != status {
		t.Fatalf("check %s status = %q, want %q; message=%q", name, check.Status, status, check.Message)
	}
}

func findCheck(report Report, name string) Check {
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	return Check{}
}
