package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider  string    `yaml:"provider" json:"provider"`
	Workspace string    `yaml:"workspace" json:"workspace"`
	Local     Local     `yaml:"local" json:"local"`
	Queue     Queue     `yaml:"queue" json:"queue"`
	Private   Private   `yaml:"private" json:"private"`
	Community Community `yaml:"community" json:"community"`
	Source    Source    `yaml:"source" json:"source"`
	Target    Target    `yaml:"target" json:"target"`
	MR        MR        `yaml:"mr" json:"mr"`
	Workflow  Workflow  `yaml:"workflow" json:"workflow"`
	Auth      Auth      `yaml:"auth" json:"auth"`
}

type Local struct {
	Path string `yaml:"path" json:"path"`
}

type Queue struct {
	Remote    string `yaml:"remote" json:"remote"`
	RemoteURL string `yaml:"remote_url" json:"remote_url"`
	Branch    string `yaml:"branch" json:"branch"`
	BaseRef   string `yaml:"base_ref" json:"base_ref"`
	StartSHA  string `yaml:"start_sha" json:"start_sha"`
	EndSHA    string `yaml:"end_sha" json:"end_sha"`
}

type Private struct {
	Remote         string `yaml:"remote" json:"remote"`
	RemoteURL      string `yaml:"remote_url" json:"remote_url"`
	BranchPrefix   string `yaml:"branch_prefix" json:"branch_prefix"`
	BranchTemplate string `yaml:"branch_template" json:"branch_template"`
	HeadNamespace  string `yaml:"head_namespace" json:"head_namespace"`
}

type Community struct {
	Remote    string `yaml:"remote" json:"remote"`
	RemoteURL string `yaml:"remote_url" json:"remote_url"`
	Owner     string `yaml:"owner" json:"owner"`
	Repo      string `yaml:"repo" json:"repo"`
	Branch    string `yaml:"branch" json:"branch"`
}

type Source struct {
	LocalPath      string `yaml:"local_path" json:"local_path"`
	Repo           string `yaml:"repo" json:"repo"`
	Branch         string `yaml:"branch" json:"branch"`
	Range          string `yaml:"range" json:"range"`
	StartSHA       string `yaml:"start_sha" json:"start_sha"`
	EndSHA         string `yaml:"end_sha" json:"end_sha"`
	BaseRef        string `yaml:"base_ref" json:"base_ref"`
	Remote         string `yaml:"remote" json:"remote"`
	BranchPrefix   string `yaml:"branch_prefix" json:"branch_prefix"`
	BranchTemplate string `yaml:"branch_template" json:"branch_template"`
	HeadNamespace  string `yaml:"head_namespace" json:"head_namespace"`
}

type Target struct {
	Owner  string `yaml:"owner" json:"owner"`
	Repo   string `yaml:"repo" json:"repo"`
	Branch string `yaml:"branch" json:"branch"`
}

type MR struct {
	BranchPrefix   string `yaml:"branch_prefix" json:"branch_prefix"`
	BranchTemplate string `yaml:"branch_template" json:"branch_template"`
}

type Workflow struct {
	CommitRange         string `yaml:"commit_range" json:"commit_range"`
	MergeMethod         string `yaml:"merge_method" json:"merge_method"`
	ReviewComment       string `yaml:"review_comment" json:"review_comment"`
	RequiredCommentText string `yaml:"required_comment_text" json:"required_comment_text"`
	Approve             *bool  `yaml:"approve" json:"approve"`
	ApprovalFailureMode string `yaml:"approval_failure_mode" json:"approval_failure_mode"`
	LoopDelayMin        string `yaml:"loop_delay_min" json:"loop_delay_min"`
	LoopDelayMax        string `yaml:"loop_delay_max" json:"loop_delay_max"`
	WaitCheckDelayMin   string `yaml:"wait_check_delay_min" json:"wait_check_delay_min"`
	WaitCheckDelayMax   string `yaml:"wait_check_delay_max" json:"wait_check_delay_max"`
	NextPRDelayMin      string `yaml:"next_pr_delay_min" json:"next_pr_delay_min"`
	NextPRDelayMax      string `yaml:"next_pr_delay_max" json:"next_pr_delay_max"`
	StopOnFailure       bool   `yaml:"stop_on_failure" json:"stop_on_failure"`
}

func (w Workflow) ShouldApprove() bool {
	return w.Approve == nil || *w.Approve
}

func (w Workflow) WarnOnApprovalFailure() bool {
	return w.ApprovalFailureMode == "warn"
}

func (w Workflow) UsesExternalMerge() bool {
	return w.MergeMethod == "external"
}

func (w Workflow) LoopDelayRange() (time.Duration, time.Duration, error) {
	return parseDelayRange(w.LoopDelayMin, w.LoopDelayMax, "1m", "5m", "workflow.loop_delay")
}

func (w Workflow) WaitCheckDelayRange() (time.Duration, time.Duration, error) {
	return parseDelayRange(w.WaitCheckDelayMin, w.WaitCheckDelayMax, "10s", "30s", "workflow.wait_check_delay")
}

func (w Workflow) NextPRDelayRange() (time.Duration, time.Duration, error) {
	minValue := w.NextPRDelayMin
	maxValue := w.NextPRDelayMax
	if minValue == "" && maxValue == "" {
		minValue = w.LoopDelayMin
		maxValue = w.LoopDelayMax
	}
	return parseDelayRange(minValue, maxValue, "1m", "5m", "workflow.next_pr_delay")
}

func parseDelayRange(minValue string, maxValue string, defaultMin string, defaultMax string, name string) (time.Duration, time.Duration, error) {
	minDelay, err := time.ParseDuration(defaultMin)
	if err != nil {
		return 0, 0, err
	}
	maxDelay, err := time.ParseDuration(defaultMax)
	if err != nil {
		return 0, 0, err
	}
	if minValue != "" {
		parsed, err := time.ParseDuration(minValue)
		if err != nil {
			return 0, 0, fmt.Errorf("parse %s_min: %w", name, err)
		}
		minDelay = parsed
	}
	if maxValue != "" {
		parsed, err := time.ParseDuration(maxValue)
		if err != nil {
			return 0, 0, fmt.Errorf("parse %s_max: %w", name, err)
		}
		maxDelay = parsed
	} else if minValue != "" {
		maxDelay = minDelay
	}
	if minDelay <= 0 || maxDelay <= 0 {
		return 0, 0, fmt.Errorf("%s delays must be positive", name)
	}
	if maxDelay < minDelay {
		return 0, 0, fmt.Errorf("%s_max must be >= %s_min", name, name)
	}
	return minDelay, maxDelay, nil
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
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
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
	safe.Queue.RemoteURL = redactURLCredential(safe.Queue.RemoteURL)
	safe.Private.RemoteURL = redactURLCredential(safe.Private.RemoteURL)
	safe.Community.RemoteURL = redactURLCredential(safe.Community.RemoteURL)
	body, err := json.MarshalIndent(safe, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(body)
}

func redactURLCredential(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}
	parsed.User = nil
	return parsed.String()
}

func (c *Config) applySimpleMode() {
	if c.Provider == "" {
		c.Provider = inferProvider(c.Source.Repo, c.Target.Repo)
	}
	if c.Workspace != "" {
		if c.Local.Path == "" {
			c.Local.Path = c.Workspace
		}
		if c.Source.LocalPath == "" {
			c.Source.LocalPath = c.Workspace
		}
	}
	sourceRepo, sourceOK := parseRepoRef(c.Provider, c.Source.Repo)
	if sourceOK {
		if c.Queue.Remote == "" {
			c.Queue.Remote = sourceRepo.remoteName()
		}
		if c.Queue.RemoteURL == "" {
			c.Queue.RemoteURL = sourceRepo.cloneURL()
		}
		if c.Queue.Branch == "" {
			c.Queue.Branch = c.Source.Branch
		}
		if c.Private.Remote == "" {
			c.Private.Remote = c.Queue.Remote
		}
		if c.Private.RemoteURL == "" {
			c.Private.RemoteURL = c.Queue.RemoteURL
		}
		if c.Private.HeadNamespace == "" {
			c.Private.HeadNamespace = sourceRepo.Owner
		}
		if c.Source.Remote == "" {
			c.Source.Remote = c.Queue.Remote
		}
		if c.Source.HeadNamespace == "" {
			c.Source.HeadNamespace = sourceRepo.Owner
		}
	}
	targetRepo, targetOK := parseRepoRef(c.Provider, c.Target.Repo)
	if targetOK {
		if c.Community.Remote == "" {
			c.Community.Remote = targetRepo.remoteName()
		}
		if c.Community.RemoteURL == "" {
			c.Community.RemoteURL = targetRepo.cloneURL()
		}
		if c.Community.Owner == "" {
			c.Community.Owner = targetRepo.Owner
		}
		if c.Community.Repo == "" || strings.Contains(c.Community.Repo, "/") {
			c.Community.Repo = targetRepo.Name
		}
		if c.Target.Owner == "" {
			c.Target.Owner = targetRepo.Owner
		}
	}
	if c.Target.Branch != "" && c.Community.Branch == "" {
		c.Community.Branch = c.Target.Branch
	}
	if c.Source.StartSHA != "" && c.Queue.StartSHA == "" {
		c.Queue.StartSHA = c.Source.StartSHA
	}
	if c.Source.EndSHA != "" && c.Queue.EndSHA == "" {
		c.Queue.EndSHA = c.Source.EndSHA
	}
	if c.MR.BranchPrefix != "" && c.Private.BranchPrefix == "" {
		c.Private.BranchPrefix = c.MR.BranchPrefix
	}
	if c.MR.BranchTemplate != "" && c.Private.BranchTemplate == "" {
		c.Private.BranchTemplate = c.MR.BranchTemplate
	}
}

type repoRef struct {
	Host  string
	Owner string
	Name  string
}

func parseRepoRef(provider string, value string) (repoRef, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return repoRef{}, false
	}
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	if before, after, ok := strings.Cut(value, "@"); ok {
		value = before + "/" + after
	}
	value = strings.ReplaceAll(value, ":", "/")
	parts := strings.Split(value, "/")
	if len(parts) == 2 {
		host := hostForProvider(provider)
		if host == "" {
			return repoRef{}, false
		}
		return repoRef{Host: host, Owner: parts[0], Name: parts[1]}, true
	}
	if len(parts) >= 3 {
		return repoRef{Host: parts[0], Owner: parts[len(parts)-2], Name: parts[len(parts)-1]}, true
	}
	return repoRef{}, false
}

func inferProvider(values ...string) string {
	for _, value := range values {
		repo, ok := parseRepoRef("", value)
		if !ok {
			continue
		}
		switch strings.ToLower(repo.Host) {
		case "gitcode.com":
			return "gitcode"
		case "atomgit.com":
			return "atomgit"
		}
	}
	return ""
}

func hostForProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gitcode":
		return "gitcode.com"
	case "atomgit":
		return "atomgit.com"
	default:
		return ""
	}
}

func (r repoRef) cloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s.git", r.Host, r.Owner, r.Name)
}

func (r repoRef) remoteName() string {
	return "mrq-" + slugify(strings.Join([]string{r.Host, r.Owner, r.Name}, "-"))
}

func remoteURLFor(provider string, owner string, repo string) string {
	if owner == "" || repo == "" {
		return ""
	}
	host := hostForProvider(provider)
	if host == "" {
		return ""
	}
	return repoRef{Host: host, Owner: owner, Name: repo}.cloneURL()
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugPattern.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func (c *Config) applyDefaults() {
	c.applySimpleMode()
	if c.Local.Path == "" {
		c.Local.Path = c.Source.LocalPath
	}
	if c.Local.Path == "" {
		c.Local.Path = c.Workspace
	}
	if c.Private.Remote == "" {
		c.Private.Remote = c.Source.Remote
	}
	if c.Private.RemoteURL == "" {
		c.Private.RemoteURL = c.Queue.RemoteURL
	}
	if c.Private.BranchPrefix == "" {
		c.Private.BranchPrefix = c.Source.BranchPrefix
	}
	if c.Private.BranchTemplate == "" {
		c.Private.BranchTemplate = c.Source.BranchTemplate
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
	if c.Community.RemoteURL == "" {
		c.Community.RemoteURL = remoteURLFor(c.Provider, c.Community.Owner, c.Community.Repo)
	}
	if c.Queue.Remote == "" {
		c.Queue.Remote = c.Private.Remote
	}
	if c.Queue.RemoteURL == "" {
		c.Queue.RemoteURL = c.Private.RemoteURL
	}
	if c.Queue.Branch == "" {
		c.Queue.Branch = c.Source.Branch
	}
	if c.Queue.StartSHA == "" {
		c.Queue.StartSHA = c.Source.StartSHA
	}
	if c.Queue.EndSHA == "" {
		c.Queue.EndSHA = c.Source.EndSHA
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
	if c.Workspace == "" {
		c.Workspace = c.Local.Path
	}
	if c.Source.Remote == "" {
		c.Source.Remote = c.Private.Remote
	}
	if c.Source.BranchPrefix == "" {
		c.Source.BranchPrefix = c.Private.BranchPrefix
	}
	if c.Source.BranchTemplate == "" {
		c.Source.BranchTemplate = c.Private.BranchTemplate
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
	if c.Private.BranchTemplate == "" {
		c.Private.BranchTemplate = "{prefix}-{sha12}"
		c.Source.BranchTemplate = "{prefix}-{sha12}"
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
	if c.Workflow.ApprovalFailureMode == "" {
		c.Workflow.ApprovalFailureMode = "fail"
	}
	if c.Workflow.LoopDelayMin == "" {
		c.Workflow.LoopDelayMin = "1m"
	}
	if c.Workflow.LoopDelayMax == "" {
		c.Workflow.LoopDelayMax = "5m"
	}
	if c.Workflow.WaitCheckDelayMin == "" {
		c.Workflow.WaitCheckDelayMin = "10s"
	}
	if c.Workflow.WaitCheckDelayMax == "" {
		c.Workflow.WaitCheckDelayMax = "30s"
	}
	if c.Workflow.NextPRDelayMin == "" {
		c.Workflow.NextPRDelayMin = c.Workflow.LoopDelayMin
	}
	if c.Workflow.NextPRDelayMax == "" {
		c.Workflow.NextPRDelayMax = c.Workflow.LoopDelayMax
	}
	if c.Workflow.CommitRange == "" && c.Source.Range != "" {
		c.Workflow.CommitRange = c.Source.Range
	}
	if c.Workflow.CommitRange == "" && c.Queue.StartSHA != "" && c.Queue.EndSHA != "" {
		c.Workflow.CommitRange = c.Queue.StartSHA + "^.." + c.Queue.EndSHA
	}
	if c.Workflow.CommitRange == "" && c.Queue.Remote != "" && c.Queue.Branch != "" && c.Queue.BaseRef != "" {
		c.Workflow.CommitRange = c.Queue.BaseRef + ".." + c.Queue.Remote + "/" + c.Queue.Branch
	}
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Source.Repo) != "" {
		if !repoHasHost(c.Source.Repo) {
			return fmt.Errorf("source.repo must include repository host, for example gitcode.com/owner/repo")
		}
		if strings.TrimSpace(c.Source.Branch) == "" && strings.TrimSpace(c.Source.Range) == "" && (strings.TrimSpace(c.Source.StartSHA) == "" || strings.TrimSpace(c.Source.EndSHA) == "") {
			return fmt.Errorf("source.branch is required unless source.range or both source.start_sha and source.end_sha are set")
		}
	}
	if strings.TrimSpace(c.Source.Repo) != "" && strings.TrimSpace(c.Target.Repo) != "" && !repoHasHost(c.Target.Repo) {
		return fmt.Errorf("target.repo must include repository host, for example gitcode.com/owner/repo")
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "source.repo", value: c.Source.Repo},
		{name: "target.repo", value: c.Target.Repo},
	} {
		if err := validateRepoHost(c.Provider, item.name, item.value); err != nil {
			return err
		}
	}
	return nil
}

func repoHasHost(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	parts := strings.Split(value, "/")
	return len(parts) >= 3
}

func validateRepoHost(provider string, name string, value string) error {
	repo, ok := parseRepoRef(provider, value)
	if !ok {
		return nil
	}
	if strings.EqualFold(repo.Host, "gitcode.com") {
		return nil
	}
	if strings.EqualFold(repo.Host, "atomgit.com") {
		return nil
	}
	return fmt.Errorf("%s uses unsupported repository host %s; only gitcode.com and atomgit.com are supported", name, repo.Host)
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
