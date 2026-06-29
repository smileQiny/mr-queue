package app

import (
	"fmt"
	"path/filepath"

	"mr-queue/internal/config"
	"mr-queue/internal/gitcode"
	"mr-queue/internal/runner"
	"mr-queue/internal/state"
)

type Runtime struct {
	Config *config.Config
	State  *state.Store
	Runner *runner.Runner
}

func Build(configPath string, envPath string, statePath string) (*Runtime, error) {
	cfg, err := config.Load(configPath, envPath)
	if err != nil {
		return nil, err
	}
	if statePath == "" {
		statePath = filepath.Join(cfg.Local.Path, ".mr-queue", "state.json")
	}
	store, err := state.Open(statePath)
	if err != nil {
		return nil, err
	}
	submitter := gitcode.NewClientForProvider(cfg.Provider, cfg.Auth.Submitter.Token)
	reviewerToken := cfg.Auth.Reviewer.Token
	if reviewerToken == "" {
		reviewerToken = cfg.Auth.Maintainer.Token
	}
	maintainerToken := cfg.Auth.Maintainer.Token
	if reviewerToken == "" {
		return nil, fmt.Errorf("reviewer token is required")
	}
	if maintainerToken == "" && !cfg.Workflow.UsesExternalMerge() {
		return nil, fmt.Errorf("maintainer token is required unless workflow.merge_method is external")
	}
	r := runner.New(
		cfg,
		store,
		runner.LocalGitOps{
			Dir:            cfg.Local.Path,
			Username:       cfg.Private.HeadNamespace,
			AccessToken:    cfg.Auth.Submitter.Token,
			ManagedRemotes: managedRemotes(cfg),
		},
		submitter,
		gitcode.NewClientForProvider(cfg.Provider, reviewerToken),
		gitcode.NewClientForProvider(cfg.Provider, maintainerToken),
	)
	return &Runtime{Config: &cfg, State: store, Runner: r}, nil
}

func managedRemotes(cfg config.Config) map[string]string {
	remotes := map[string]string{}
	if cfg.Queue.Remote != "" && cfg.Queue.RemoteURL != "" {
		remotes[cfg.Queue.Remote] = cfg.Queue.RemoteURL
	}
	if cfg.Private.Remote != "" && cfg.Private.RemoteURL != "" {
		remotes[cfg.Private.Remote] = cfg.Private.RemoteURL
	}
	if cfg.Community.Remote != "" && cfg.Community.RemoteURL != "" {
		remotes[cfg.Community.Remote] = cfg.Community.RemoteURL
	}
	if len(remotes) == 0 {
		return nil
	}
	return remotes
}
