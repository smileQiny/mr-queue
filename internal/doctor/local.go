package doctor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"mr-queue/internal/runner"
)

type LocalGitChecker struct {
	Dir         string
	Username    string
	AccessToken string
}

func (g LocalGitChecker) IsRepository(path string) bool {
	if path == "" {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = filepath.Clean(path)
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func (g LocalGitChecker) RemoteState(name string, desiredURL string) (RemoteState, error) {
	out, err := g.git("remote", "get-url", name)
	if err != nil {
		if strings.Contains(err.Error(), "No such remote") || strings.Contains(err.Error(), "No such remote '") {
			return RemoteState{Name: name, DesiredURL: desiredURL, Exists: false}, nil
		}
		return RemoteState{}, err
	}
	return RemoteState{Name: name, URL: strings.TrimSpace(out), DesiredURL: desiredURL, Exists: true}, nil
}

func (g LocalGitChecker) EnsureRemote(name string, desiredURL string) error {
	state, err := g.RemoteState(name, desiredURL)
	if err != nil {
		return err
	}
	if !state.Exists {
		return g.run("remote", "add", name, desiredURL)
	}
	if strings.TrimSpace(state.URL) == strings.TrimSpace(desiredURL) {
		return nil
	}
	return g.run("remote", "set-url", name, desiredURL)
}

func (g LocalGitChecker) Fetch(remote string) error {
	return g.run("fetch", "--prune", remote)
}

func (g LocalGitChecker) CheckCommitRange(commitRange string) error {
	_, err := g.git("log", "--oneline", "-1", commitRange)
	return err
}

func (g LocalGitChecker) git(args ...string) (string, error) {
	ops := runner.LocalGitOps{Dir: g.Dir, Username: g.Username, AccessToken: g.AccessToken}
	out, err := ops.GitForDoctor(args...)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return out, nil
}

func (g LocalGitChecker) run(args ...string) error {
	_, err := g.git(args...)
	return err
}
