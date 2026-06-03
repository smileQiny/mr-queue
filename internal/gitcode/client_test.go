package gitcode

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreatePullUsesGitCodeV5EndpointAndTokenParam(t *testing.T) {
	var seenPath string
	var seenToken string
	var payload PullRequestInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenToken = r.URL.Query().Get("access_token")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"html_url":"https://gitcode.com/x/y/pulls/42"}`))
	}))
	defer server.Close()

	client := NewClient("token-1")
	client.BaseURL = server.URL + "/api/v5"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/merge") {
			if err := json.NewDecoder(r.Body).Decode(&mergePayload); err != nil {
				t.Fatal(err)
			}
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("token-2")
	client.BaseURL = server.URL + "/api/v5"
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
