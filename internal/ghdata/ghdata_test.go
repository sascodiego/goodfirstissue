package ghdata

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const fixture = `{"data":{"repository":{
  "nameWithOwner":"spf13/cobra","diskUsage":2107,
  "primaryLanguage":{"name":"Go"},
  "defaultBranchRef":{"name":"main","target":{"oid":"adbc8813901b"}},
  "issues":{"totalCount":262,"pageInfo":{"hasNextPage":false,"endCursor":"c1"},
   "nodes":[{
    "number":1391,"title":"404 links","body":"body","url":"https://x/1391",
    "state":"OPEN","stateReason":null,"authorAssociation":"NONE",
    "createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-02-01T00:00:00Z",
    "author":{"login":"someone"},
    "assignees":{"nodes":[{"login":"maint"}]},
    "labels":{"nodes":[{"name":"good-first-issue"},{"name":"\u2709 help wanted"}]},
    "comments":{"totalCount":3,"nodes":[
      {"author":{"login":"m"},"body":"hi","createdAt":"2026-01-02T00:00:00Z","authorAssociation":"MEMBER"}]},
    "timelineItems":{"nodes":[
      {"__typename":"CrossReferencedEvent","source":{"__typename":"PullRequest","number":1984,"state":"OPEN","url":"u1","merged":false}},
      {"__typename":"ConnectedEvent","subject":{"__typename":"PullRequest","number":2000,"state":"MERGED","url":"u2","merged":true}},
      {"__typename":"ConnectedEvent","subject":{"__typename":"Issue","number":5}}]}
   }]}}}}`

func TestDecodeKeepsLabelsVerbatimAndFindsLinkedPRs(t *testing.T) {
	got, err := decode([]byte(fixture))
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if got.NameWithOwner != "spf13/cobra" || got.CommitSHA != "adbc8813901b" {
		t.Errorf("repository = %+v, want the pinned identity", got.Repository)
	}
	if got.TotalOpen != 262 {
		t.Errorf("TotalOpen = %d, want 262", got.TotalOpen)
	}
	if len(got.Issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(got.Issues))
	}

	iss := got.Issues[0]
	// Labels are text, not an enum: they must arrive exactly as written.
	if len(iss.Labels) != 2 || iss.Labels[0] != "good-first-issue" || iss.Labels[1] != "✉ help wanted" {
		t.Errorf("labels = %q, want them verbatim", iss.Labels)
	}
	if iss.CommentCount != 3 || len(iss.Comments) != 1 {
		t.Errorf("comments: total=%d fetched=%d, want 3/1", iss.CommentCount, len(iss.Comments))
	}
	if iss.Author != "someone" || iss.Comments[0].AuthorAssociation != "MEMBER" {
		t.Errorf("authors not carried: %+v", iss)
	}
	// Both event shapes carry a PR; a non-PR subject must be ignored.
	if len(iss.LinkedPRs) != 2 {
		t.Fatalf("LinkedPRs = %+v, want 2", iss.LinkedPRs)
	}
	if iss.LinkedPRs[0].Number != 1984 || iss.LinkedPRs[0].State != "OPEN" {
		t.Errorf("first PR = %+v", iss.LinkedPRs[0])
	}
	if !iss.LinkedPRs[1].Merged {
		t.Errorf("merged flag lost: %+v", iss.LinkedPRs[1])
	}
}

func TestDecodeReportsGraphQLErrors(t *testing.T) {
	_, err := decode([]byte(`{"errors":[{"message":"Could not resolve to a Repository"}]}`))
	if err == nil || !strings.Contains(err.Error(), "Could not resolve") {
		t.Fatalf("err = %v, want the GraphQL message", err)
	}
}

// The only filter on an issue is its state. A label filter would be a design defect.
func TestQueryFiltersOnlyByOpenState(t *testing.T) {
	if !strings.Contains(query, "states: OPEN") {
		t.Error("the query does not filter by open state")
	}
	if strings.Contains(query, "issues(labels:") {
		t.Error("the query filters by label, which the product forbids")
	}
}

func TestFetchSendsOwnerNameAndPage(t *testing.T) {
	var gotArgs []string
	c := &Client{Run: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(fixture), nil
	}}

	snap, err := c.Fetch(t.Context(), "spf13", "cobra", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snap.Issues) != 1 {
		t.Fatalf("got %d issues", len(snap.Issues))
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"api graphql", "owner=spf13", "name=cobra", "page=100"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "cursor=") {
		t.Errorf("first page must not send a cursor: %q", joined)
	}
}

func TestFetchPaginatesWithCursor(t *testing.T) {
	page1 := strings.Replace(fixture, `"hasNextPage":false,"endCursor":"c1"`, `"hasNextPage":true,"endCursor":"CUR"`, 1)
	var calls []string
	c := &Client{Run: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		if len(calls) == 1 {
			return []byte(page1), nil
		}
		return []byte(fixture), nil
	}}

	snap, err := c.Fetch(t.Context(), "spf13", "cobra", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("made %d calls, want 2", len(calls))
	}
	if !strings.Contains(calls[1], "cursor=CUR") {
		t.Errorf("second call missing the cursor: %q", calls[1])
	}
	if len(snap.Issues) != 2 {
		t.Errorf("got %d issues, want both pages", len(snap.Issues))
	}
}

func TestFetchStopsAtMaxIssues(t *testing.T) {
	page1 := strings.Replace(fixture, `"hasNextPage":false,"endCursor":"c1"`, `"hasNextPage":true,"endCursor":"CUR"`, 1)
	c := &Client{Run: func(context.Context, []byte, ...string) ([]byte, error) {
		return []byte(page1), nil
	}}

	snap, err := c.Fetch(t.Context(), "spf13", "cobra", Options{MaxIssues: 1})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snap.Issues) != 1 {
		t.Errorf("got %d issues, want the cap of 1", len(snap.Issues))
	}
	if !snap.Truncated {
		t.Error("Truncated = false, want true when the cap stopped the fetch")
	}
}

func TestFetchReportsEmptyRepository(t *testing.T) {
	empty := `{"data":{"repository":{"nameWithOwner":"a/b","issues":{"totalCount":0,"pageInfo":{"hasNextPage":false},"nodes":[]}}}}`
	c := &Client{Run: func(context.Context, []byte, ...string) ([]byte, error) { return []byte(empty), nil }}

	_, err := c.Fetch(t.Context(), "a", "b", Options{})
	if !errors.Is(err, ErrNoIssues) {
		t.Fatalf("err = %v, want ErrNoIssues", err)
	}
}

func TestFetchValidatesInput(t *testing.T) {
	c := &Client{}
	if _, err := c.Fetch(t.Context(), "", "b", Options{}); err == nil {
		t.Fatal("want an error for a missing owner")
	}
}

// A repository that does not exist is not a failure of ours: the web layer answers 400
// for it and 500 only for what actually broke. GraphQL says which one it is with the
// error type, so the distinction is carried as a sentinel instead of by matching prose.
func TestDecodeTellsAMissingRepositoryFromAnyOtherFailure(t *testing.T) {
	missing := []byte(`{"errors":[{"type":"NOT_FOUND","path":["repository"],
		"message":"Could not resolve to a Repository with the name 'nope/nope'."}]}`)
	if _, err := decode(missing); !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("err = %v, want ErrRepoNotFound", err)
	}

	// The negative control: any other GraphQL failure must stay a plain error, or the
	// handler would answer 400 to a rate limit and hide a real problem.
	other := []byte(`{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`)
	_, err := decode(other)
	if err == nil {
		t.Fatal("want an error for a rate limit")
	}
	if errors.Is(err, ErrRepoNotFound) {
		t.Errorf("err = %v, must not be ErrRepoNotFound", err)
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("err = %v, want the GraphQL message kept", err)
	}
}
