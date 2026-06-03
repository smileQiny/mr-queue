package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Local     Local     `yaml:"local" json:"local"`
	Queue     Queue     `yaml:"queue" json:"queue"`
	Private   Private   `yaml:"private" json:"private"`
	Community Community `yaml:"community" json:"community"`
	Source    Source    `yaml:"source" json:"source"`
	Target    Target    `yaml:"target" json:"target"`
	Workflow  Workflow  `yaml:"workflow" json:"workflow"`
	Auth      Auth      `yaml:"auth" json:"auth"`
}

type Local struct {
	Path string `yaml:"path" json:"path"`
}

type Queue struct {
	Remote  string `yaml:"remote" json:"remote"`
	Branch  string `yaml:"branch" json:"branch"`
	BaseRef string `yaml:"base_ref" json:"base_ref"`
}

type Private struct {
	Remote        string `yaml:"remote" json:"remote"`
	BranchPrefix  string `yaml:"branch_prefix" json:"branch_prefix"`
	HeadNamespace string `yaml:"head_namespace" json:"head_namespace"`
}

type Community struct {
	Remote string `yaml:"remote" json:"remote"`
	Owner  string `yaml:"owner" json:"owner"`
	Repo   string `yaml:"repo" json:"repo"`
	Branch string `yaml:"branch" json:"branch"`
}

type Source struct {
	LocalPath     string `yaml:"local_path" json:"local_path"`
	BaseRef       string `yaml:"base_ref" json:"base_ref"`
	Remote        string `yaml:"remote" json:"remote"`
	BranchPrefix  string `yaml:"branch_prefix" json:"branch_prefix"`
	HeadNamespace string `yaml:"head_namespace" json:"head_namespace"`
}

type Target struct {
	Owner  string `yaml:"owner" json:"owner"`
	Repo   string `yaml:"repo" json:"repo"`
	Branch string `yaml:"branch" json:"branch"`
}

type Workflow struct {
	CommitRange   string `yaml:"commit_range" json:"commit_range"`
	MergeMethod   string `yaml:"merge_method" json:"merge_method"`
	ReviewComment string `yaml:"review_comment" json:"review_comment"`
	StopOnFailure bool   `yaml:"stop_on_failure" json:"stop_on_failure"`
}

type Auth struct {
	Submitter  Credential `yaml:"submitter" json:"submitter"`
	Reviewer   Credential `yaml:"reviewer" json:"reviewer"`
	Maintainer Credential `yaml:"maintainer" json:"maintainer"`
}

type Credential struct {
	TokenEnv string `yaml:"token_env" json:"token_env"`
	Token    string `yaml:"-" json:"-"`
}

func Load(configPath string, envPath string) (Config, error) {
	if envPath != "" {
		if err := loadDotenv(envPath); err != nil {
			return Config{}, err
		}
	} else {
		defaultEnv := filepath.Join(filepath.Dir(configPath), ".env")
		if _, err := os.Stat(defaultEnv); err == nil {
			if err := loadDotenv(defaultEnv); err != nil {
				return Config{}, err
			}
		}
	}

	body, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", configPath, err)
	}
	cfg.applyDefaults()
	if err := cfg.resolveTokens(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Safe() string {
	safe := c
	safe.Auth.Submitter.Token = ""
	safe.Auth.Reviewer.Token = ""
	safe.Auth.Maintainer.Token = ""
	body, err := json.MarshalIndent(safe, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(body)
}

func (c *Config) applyDefaults() {
	if c.Local.Path == "" {
		c.Local.Path = c.Source.LocalPath
	}
	if c.Private.Remote == "" {
		c.Private.Remote = c.Source.Remote
	}
	if c.Private.BranchPrefix == "" {
		c.Private.BranchPrefix = c.Source.BranchPrefix
	}
	if c.Private.HeadNamespace == "" {
		c.Private.HeadNamespace = c.Source.HeadNamespace
	}
	if c.Community.Owner == "" {
		c.Community.Owner = c.Target.Owner
	}
	if c.Community.Repo == "" {
		c.Community.Repo = c.Target.Repo
	}
	if c.Community.Branch == "" {
		c.Community.Branch = c.Target.Branch
	}
	if c.Queue.Remote == "" {
		c.Queue.Remote = c.Private.Remote
	}
	if c.Queue.BaseRef == "" {
		if c.Community.Remote != "" && c.Community.Branch != "" {
			c.Queue.BaseRef = c.Community.Remote + "/" + c.Community.Branch
		} else {
			c.Queue.BaseRef = c.Source.BaseRef
		}
	}
	if c.Source.LocalPath == "" {
		c.Source.LocalPath = c.Local.Path
	}
	if c.Source.LocalPath == "" {
		c.Source.LocalPath = "."
	}
	if c.Local.Path == "" {
		c.Local.Path = c.Source.LocalPath
	}
	if c.Source.Remote == "" {
		c.Source.Remote = c.Private.Remote
	}
	if c.Source.BranchPrefix == "" {
		c.Source.BranchPrefix = c.Private.BranchPrefix
	}
	if c.Source.HeadNamespace == "" {
		c.Source.HeadNamespace = c.Private.HeadNamespace
	}
	if c.Source.BaseRef == "" {
		c.Source.BaseRef = c.Queue.BaseRef
	}
	if c.Private.Remote == "" {
		c.Private.Remote = "origin"
		c.Source.Remote = "origin"
	}
	if c.Private.BranchPrefix == "" {
		c.Private.BranchPrefix = "mr-queue"
		c.Source.BranchPrefix = "mr-queue"
	}
	if c.Target.Owner == "" {
		c.Target.Owner = c.Community.Owner
	}
	if c.Target.Repo == "" {
		c.Target.Repo = c.Community.Repo
	}
	if c.Target.Branch == "" {
		c.Target.Branch = c.Community.Branch
	}
	if c.Community.Branch == "" {
		c.Community.Branch = "master"
		c.Target.Branch = "master"
	}
	if c.Workflow.MergeMethod == "" {
		c.Workflow.MergeMethod = "squash"
	}
	if c.Workflow.ReviewComment == "" {
		c.Workflow.ReviewComment = "已完成预审，确认合并。"
	}
	if c.Workflow.CommitRange == "" && c.Queue.Remote != "" && c.Queue.Branch != "" && c.Queue.BaseRef != "" {
		c.Workflow.CommitRange = c.Queue.BaseRef + ".." + c.Queue.Remote + "/" + c.Queue.Branch
	}
}

func (c *Config) resolveTokens() error {
	if err := resolveCredential("auth.submitter", &c.Auth.Submitter); err != nil {
		return err
	}
	if c.Auth.Reviewer.TokenEnv != "" {
		if err := resolveCredential("auth.reviewer", &c.Auth.Reviewer); err != nil {
			return err
		}
	}
	if c.Auth.Maintainer.TokenEnv != "" {
		if err := resolveCredential("auth.maintainer", &c.Auth.Maintainer); err != nil {
			return err
		}
	}
	return nil
}

func resolveCredential(name string, cred *Credential) error {
	if cred.TokenEnv == "" {
		return fmt.Errorf("%s.token_env is required", name)
	}
	token := os.Getenv(cred.TokenEnv)
	if token == "" {
		return fmt.Errorf("environment variable %s is required for %s", cred.TokenEnv, name)
	}
	cred.Token = token
	return nil
}

func loadDotenv(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read env file %s: %w", path, err)
	}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line in %s: %q", path, line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return fmt.Errorf("invalid empty env key in %s", path)
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}
