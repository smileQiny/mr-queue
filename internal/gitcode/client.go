package gitcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	token   string
	http    *http.Client
}

type PullRequestInput struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body,omitempty"`
}

type PullRequest struct {
	Number         int    `json:"number"`
	HTMLURL        string `json:"html_url"`
	State          string `json:"state"`
	Merged         bool   `json:"merged"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	HeadSHA        string `json:"head_sha"`
}

type Comment struct {
	ID   StringID `json:"id"`
	Body string   `json:"body"`
}

type Review struct{}

type StringID string

func (id *StringID) UnmarshalJSON(body []byte) error {
	var text string
	if err := json.Unmarshal(body, &text); err == nil {
		*id = StringID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(body, &number); err != nil {
		return err
	}
	*id = StringID(number.String())
	return nil
}

type MergeInput struct {
	MergeMethod string `json:"merge_method,omitempty"`
}

type MergeResult struct {
	Merged bool   `json:"merged"`
	SHA    string `json:"sha"`
}

func NewClient(token string) *Client {
	return &Client{
		BaseURL: "https://api.gitcode.com/api/v5",
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) CreatePull(owner string, repo string, input PullRequestInput) (PullRequest, error) {
	var out PullRequest
	err := c.request(http.MethodPost, pathFor(owner, repo, "pulls"), input, &out)
	return out, err
}

func (c *Client) GetPull(owner string, repo string, number int) (PullRequest, error) {
	var out PullRequest
	err := c.request(http.MethodGet, pathFor(owner, repo, fmt.Sprintf("pulls/%d", number)), nil, &out)
	return out, err
}

func (c *Client) CheckRepository(owner string, repo string, _ string) error {
	var out map[string]interface{}
	return c.request(http.MethodGet, fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), nil, &out)
}

func (c *Client) CommentPull(owner string, repo string, number int, body string) (Comment, error) {
	var out Comment
	payload := map[string]string{"body": body}
	err := c.request(http.MethodPost, pathFor(owner, repo, fmt.Sprintf("pulls/%d/comments", number)), payload, &out)
	return out, err
}

func (c *Client) ListPullComments(owner string, repo string, number int) ([]Comment, error) {
	var out []Comment
	err := c.request(http.MethodGet, pathFor(owner, repo, fmt.Sprintf("pulls/%d/comments", number)), nil, &out)
	return out, err
}

func (c *Client) ReviewPull(owner string, repo string, number int) (Review, error) {
	var out Review
	err := c.request(http.MethodPost, pathFor(owner, repo, fmt.Sprintf("pulls/%d/review", number)), map[string]string{"state": "approved"}, &out)
	return out, err
}

func (c *Client) MergePull(owner string, repo string, number int, input MergeInput) (MergeResult, error) {
	var out MergeResult
	err := c.request(http.MethodPut, pathFor(owner, repo, fmt.Sprintf("pulls/%d/merge", number)), input, &out)
	return out, err
}

func (c *Client) request(method string, path string, payload interface{}, out interface{}) error {
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(body)
	}
	endpoint, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + path)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("access_token", c.token)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(method, endpoint.String(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitcode api %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if len(respBody) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode gitcode response: %w", err)
	}
	return nil
}

func pathFor(owner string, repo string, suffix string) string {
	return fmt.Sprintf("/repos/%s/%s/%s", url.PathEscape(owner), url.PathEscape(repo), suffix)
}
