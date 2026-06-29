package gitcode

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCreatePullUsesGitCodeV5EndpointAndTokenParam(t *testing.T) {
	var seenPath string
	var seenToken string
	var payload PullRequestInput
	client := NewClient("token-1")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.Path
		seenToken = req.URL.Query().Get("access_token")
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(http.StatusCreated, `{"number":42,"html_url":"https://gitcode.com/x/y/pulls/42"}`), nil
	})}

	pr, err := client.CreatePull("community", "project", PullRequestInput{
		Title: "Add docs",
		Head:  "submitter:mr-queue-abc123",
		Base:  "master",
		Body:  "commit body",
	})
	if err != nil {
		t.Fatalf("CreatePull returned error: %v", err)
	}

	if seenPath != "/api/v5/repos/community/project/pulls" {
		t.Fatalf("path = %q", seenPath)
	}
	if seenToken != "token-1" {
		t.Fatalf("token = %q", seenToken)
	}
	if payload.Head != "submitter:mr-queue-abc123" {
		t.Fatalf("head = %q", payload.Head)
	}
	if pr.Number != 42 {
		t.Fatalf("number = %d", pr.Number)
	}
}

func TestCommentReviewAndMergeCallExpectedEndpoints(t *testing.T) {
	var paths []string
	var mergePayload MergeInput
	client := NewClient("token-2")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		if strings.HasSuffix(req.URL.Path, "/merge") {
			if err := json.NewDecoder(req.Body).Decode(&mergePayload); err != nil {
				t.Fatal(err)
			}
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})}

	if _, err := client.CommentPull("community", "project", 42, "Reviewed"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReviewPull("community", "project", 42); err != nil {
		t.Fatal(err)
	}
	if _, err := client.MergePull("community", "project", 42, MergeInput{MergeMethod: "squash"}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"/api/v5/repos/community/project/pulls/42/comments",
		"/api/v5/repos/community/project/pulls/42/review",
		"/api/v5/repos/community/project/pulls/42/merge",
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
	if mergePayload.MergeMethod != "squash" {
		t.Fatalf("merge method = %q", mergePayload.MergeMethod)
	}
}

func TestListPullCommentsUsesExpectedEndpoint(t *testing.T) {
	var seenPath string
	client := NewClient("token-5")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.Path
		return jsonResponse(http.StatusOK, `[{"id":"1","body":"CLA Signature Pass"}]`), nil
	})}

	comments, err := client.ListPullComments("community", "project", 42)
	if err != nil {
		t.Fatalf("ListPullComments returned error: %v", err)
	}

	if seenPath != "/api/v5/repos/community/project/pulls/42/comments" {
		t.Fatalf("path = %q", seenPath)
	}
	if len(comments) != 1 || comments[0].Body != "CLA Signature Pass" {
		t.Fatalf("comments = %#v", comments)
	}
}

func TestGetPullUsesExpectedEndpoint(t *testing.T) {
	var seenPath string
	client := NewClient("token-6")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.Path
		return jsonResponse(http.StatusOK, `{"number":42,"state":"merged","merged":true,"merge_commit_sha":"abc123"}`), nil
	})}

	pr, err := client.GetPull("community", "project", 42)
	if err != nil {
		t.Fatalf("GetPull returned error: %v", err)
	}

	if seenPath != "/api/v5/repos/community/project/pulls/42" {
		t.Fatalf("path = %q", seenPath)
	}
	if !pr.Merged || pr.State != "merged" || pr.MergeCommitSHA != "abc123" {
		t.Fatalf("pull = %#v", pr)
	}
}

func TestCheckRepositoryUsesExpectedEndpoint(t *testing.T) {
	var seenPath string
	client := NewClient("token-7")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.Path
		return jsonResponse(http.StatusOK, `{"full_name":"community/project"}`), nil
	})}

	if err := client.CheckRepository("community", "project", "ignored-token"); err != nil {
		t.Fatalf("CheckRepository returned error: %v", err)
	}

	if seenPath != "/api/v5/repos/community/project" {
		t.Fatalf("path = %q", seenPath)
	}
}

func TestCommentPullAcceptsStringID(t *testing.T) {
	client := NewClient("token-3")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"123"}`), nil
	})}

	comment, err := client.CommentPull("community", "project", 42, "Reviewed")
	if err != nil {
		t.Fatalf("CommentPull returned error: %v", err)
	}
	if string(comment.ID) != "123" {
		t.Fatalf("comment id = %q", comment.ID)
	}
}

func TestCommentPullAcceptsNumericID(t *testing.T) {
	client := NewClient("token-4")
	client.BaseURL = "https://example.test/api/v5"
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":456}`), nil
	})}

	comment, err := client.CommentPull("community", "project", 42, "Reviewed")
	if err != nil {
		t.Fatalf("CommentPull returned error: %v", err)
	}
	if string(comment.ID) != "456" {
		t.Fatalf("comment id = %q", comment.ID)
	}
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
