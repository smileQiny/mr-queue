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
	submitter := gitcode.NewClient(cfg.Auth.Submitter.Token)
	reviewerToken := cfg.Auth.Reviewer.Token
	if reviewerToken == "" {
		reviewerToken = cfg.Auth.Maintainer.Token
	}
	maintainerToken := cfg.Auth.Maintainer.Token
	if maintainerToken == "" {
		maintainerToken = reviewerToken
	}
	if reviewerToken == "" || maintainerToken == "" {
		return nil, fmt.Errorf("reviewer and maintainer tokens are required")
	}
	r := runner.New(
		cfg,
		store,
		runner.LocalGitOps{Dir: cfg.Local.Path},
		submitter,
		gitcode.NewClient(reviewerToken),
		gitcode.NewClient(maintainerToken),
	)
	return &Runtime{Config: &cfg, State: store, Runner: r}, nil
}
