package doctor

import (
	"fmt"
	"strings"

	"mr-queue/internal/config"
)

type Status string

const (
	StatusOK    Status = "ok"
	StatusWarn  Status = "warn"
	StatusError Status = "error"
)

type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

type Options struct {
	Fix bool
}

type GitChecker interface {
	IsRepository(path string) bool
	RemoteState(name string, desiredURL string) (RemoteState, error)
	EnsureRemote(name string, desiredURL string) error
	Fetch(remote string) error
	CheckCommitRange(commitRange string) error
}

type APIChecker interface {
	CheckRepository(owner string, repo string, token string) error
}

type RemoteState struct {
	Name       string
	URL        string
	DesiredURL string
	Exists     bool
}

func Run(cfg config.Config, options Options, git GitChecker, api APIChecker) Report {
	report := Report{OK: true}
	add := func(name string, status Status, message string, fix string) {
		report.Checks = append(report.Checks, Check{Name: name, Status: status, Message: message, Fix: fix})
		if status == StatusError {
			report.OK = false
		}
	}

	add("config", StatusOK, "configuration loaded", "")
	workspaceOK := false
	if cfg.Local.Path == "" {
		add("workspace", StatusError, "workspace path is empty", "set workspace or local.path in mr-queue.yml")
	} else if !git.IsRepository(cfg.Local.Path) {
		add("workspace", StatusError, fmt.Sprintf("%s is not a git repository", cfg.Local.Path), "set workspace to an existing git repository")
	} else {
		workspaceOK = true
		add("workspace", StatusOK, cfg.Local.Path+" is a git repository", "")
	}

	checkToken(add, "auth.submitter", cfg.Auth.Submitter, true)
	reviewerToken := cfg.Auth.Reviewer.Token
	if reviewerToken == "" {
		reviewerToken = cfg.Auth.Maintainer.Token
	}
	checkOptionalReviewer(add, cfg, reviewerToken)
	checkMaintainer(add, cfg)

	remoteReady := map[string]bool{}
	if workspaceOK {
		for _, remote := range managedRemotes(cfg) {
			remoteReady[remote.Name] = checkRemote(add, options, git, remote)
		}
		for _, remote := range fetchRemotes(cfg) {
			if ready, ok := remoteReady[remote]; ok && !ready {
				add("git."+remoteRole(cfg, remote)+".fetch", StatusWarn, "skipped fetch because remote "+remote+" is not prepared", "rerun doctor with --fix")
				continue
			}
			if err := git.Fetch(remote); err != nil {
				add("git."+remoteRole(cfg, remote)+".fetch", StatusError, fmt.Sprintf("cannot fetch %s: %v", remote, err), "check repository URL, network, and token permissions")
			} else {
				add("git."+remoteRole(cfg, remote)+".fetch", StatusOK, "fetched "+remote, "")
			}
		}
		if strings.TrimSpace(cfg.Workflow.CommitRange) != "" {
			if err := git.CheckCommitRange(cfg.Workflow.CommitRange); err != nil {
				add("git.commit_range", StatusError, fmt.Sprintf("cannot read commit range %s: %v", cfg.Workflow.CommitRange, err), "check source/target branches or source.range")
			} else {
				add("git.commit_range", StatusOK, cfg.Workflow.CommitRange, "")
			}
		}
	}

	if cfg.Community.Owner != "" && cfg.Community.Repo != "" && reviewerToken != "" {
		if err := api.CheckRepository(cfg.Community.Owner, cfg.Community.Repo, reviewerToken); err != nil {
			add("api.target", StatusError, fmt.Sprintf("cannot access target repository %s/%s: %v", cfg.Community.Owner, cfg.Community.Repo, err), "check reviewer token permissions")
		} else {
			add("api.target", StatusOK, "target repository API is reachable", "")
		}
	}

	return report
}

func checkToken(add func(string, Status, string, string), name string, cred config.Credential, required bool) {
	if cred.TokenEnv == "" {
		if required {
			add(name, StatusError, name+".token_env is not configured", "set token_env in mr-queue.yml")
		}
		return
	}
	if cred.Token == "" {
		status := StatusWarn
		if required {
			status = StatusError
		}
		add(name, status, "environment variable "+cred.TokenEnv+" is empty", "set "+cred.TokenEnv+" in .env or shell environment")
		return
	}
	add(name, StatusOK, "using "+cred.TokenEnv, "")
}

func checkOptionalReviewer(add func(string, Status, string, string), cfg config.Config, token string) {
	if cfg.Auth.Reviewer.TokenEnv == "" && cfg.Auth.Maintainer.TokenEnv == "" {
		add("auth.reviewer", StatusWarn, "reviewer token is not configured", "set auth.reviewer.token_env or auth.maintainer.token_env")
		return
	}
	if token == "" {
		add("auth.reviewer", StatusWarn, "reviewer token is empty", "set reviewer or maintainer token")
		return
	}
	envName := cfg.Auth.Reviewer.TokenEnv
	if envName == "" {
		envName = cfg.Auth.Maintainer.TokenEnv
	}
	add("auth.reviewer", StatusOK, "using "+envName, "")
}

func checkMaintainer(add func(string, Status, string, string), cfg config.Config) {
	if cfg.Workflow.UsesExternalMerge() {
		add("auth.maintainer", StatusOK, "not required for external merge", "")
		return
	}
	checkToken(add, "auth.maintainer", cfg.Auth.Maintainer, true)
}

func managedRemotes(cfg config.Config) []RemoteState {
	seen := map[string]bool{}
	var remotes []RemoteState
	for _, remote := range []RemoteState{
		{Name: cfg.Queue.Remote, DesiredURL: cfg.Queue.RemoteURL},
		{Name: cfg.Private.Remote, DesiredURL: cfg.Private.RemoteURL},
		{Name: cfg.Community.Remote, DesiredURL: cfg.Community.RemoteURL},
	} {
		if remote.Name == "" || remote.DesiredURL == "" || seen[remote.Name] {
			continue
		}
		seen[remote.Name] = true
		remotes = append(remotes, remote)
	}
	return remotes
}

func checkRemote(add func(string, Status, string, string), options Options, git GitChecker, desired RemoteState) bool {
	state, err := git.RemoteState(desired.Name, desired.DesiredURL)
	if err != nil {
		add("git.remote."+desired.Name, StatusError, err.Error(), "check local git remote configuration")
		return false
	}
	if state.Exists && strings.TrimSpace(state.URL) == strings.TrimSpace(desired.DesiredURL) {
		add("git.remote."+desired.Name, StatusOK, "remote "+desired.Name+" points to "+desired.DesiredURL, "")
		return true
	}
	if !options.Fix {
		if !state.Exists {
			add("git.remote."+desired.Name, StatusWarn, "remote "+desired.Name+" will be added", "rerun doctor with --fix")
			return false
		}
		add("git.remote."+desired.Name, StatusWarn, "remote "+desired.Name+" URL differs", "rerun doctor with --fix")
		return false
	}
	if err := git.EnsureRemote(desired.Name, desired.DesiredURL); err != nil {
		add("git.remote."+desired.Name, StatusError, "failed to update remote "+desired.Name+": "+err.Error(), "check git remote permissions")
		return false
	}
	add("git.remote."+desired.Name, StatusOK, "remote "+desired.Name+" prepared", "")
	return true
}

func fetchRemotes(cfg config.Config) []string {
	seen := map[string]bool{}
	var remotes []string
	for _, remote := range []string{cfg.Queue.Remote, cfg.Community.Remote} {
		if remote == "" || seen[remote] {
			continue
		}
		seen[remote] = true
		remotes = append(remotes, remote)
	}
	return remotes
}

func remoteRole(cfg config.Config, remote string) string {
	if remote == cfg.Queue.Remote {
		return "source"
	}
	if remote == cfg.Community.Remote {
		return "target"
	}
	return "remote"
}
