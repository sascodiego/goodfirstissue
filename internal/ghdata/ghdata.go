// Package ghdata fetches issues from GitHub through the gh CLI.
//
// The only filter applied to an issue is state: OPEN. Labels are carried verbatim as a
// declared attribute, never used to select: filtering by label would leave us auditing
// only the issues that already carry one, which is exactly where the problem hides.
package ghdata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrNoIssues reports a repository with nothing open. It is not a failure of the fetch.
var ErrNoIssues = errors.New("ghdata: the repository has no open issues")

// Repository is the pinned snapshot identity of one fetch.
type Repository struct {
	NameWithOwner   string
	DiskUsageKB     int
	PrimaryLanguage string
	DefaultBranch   string
	// CommitSHA is what every later result is pinned to (I4).
	CommitSHA string
}

// Comment is one reply in the thread.
type Comment struct {
	Author            string
	Body              string
	AuthorAssociation string
	CreatedAt         time.Time
}

// PullRequest is a PR that references the issue, in any state.
type PullRequest struct {
	Number int
	State  string
	URL    string
	Merged bool
}

// Issue carries the thread as GitHub returns it, plus the derived facts a rule can
// compute without a model (see PROYECTO.md §5.3).
type Issue struct {
	Number            int
	Title             string
	Body              string
	URL               string
	State             string
	StateReason       string
	Author            string
	AuthorAssociation string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// Labels are free text written by maintainers. Never normalized, never a filter.
	Labels       []string
	Assignees    []string
	CommentCount int
	Comments     []Comment
	LinkedPRs    []PullRequest
}

// Snapshot is one repository plus its open issues.
type Snapshot struct {
	Repository Repository
	TotalOpen  int
	Issues     []Issue
	// Truncated is true when MaxIssues capped the fetch.
	Truncated bool
}

// Client runs the gh CLI. The zero value uses the gh binary on PATH.
type Client struct {
	// GH overrides the binary name. Empty means "gh".
	GH string
	// Run replaces the process call. Nil means exec gh.
	Run func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)
}

// Options bounds one fetch.
type Options struct {
	// MaxIssues caps how many issues are collected. Zero means the default.
	MaxIssues int
	// PageSize is the GraphQL page size. Zero means 100.
	PageSize int
}

const (
	defaultMaxIssues = 150
	defaultPageSize  = 100
	maxPageSize      = 100
)

// Fetch collects the open issues of one repository, following pagination.
func (c *Client) Fetch(ctx context.Context, owner, name string, opts Options) (*Snapshot, error) {
	if owner == "" || name == "" {
		return nil, errors.New("ghdata: owner and name are required")
	}
	if opts.MaxIssues == 0 {
		opts.MaxIssues = defaultMaxIssues
	}
	if opts.PageSize == 0 || opts.PageSize > maxPageSize {
		opts.PageSize = defaultPageSize
	}

	snap := &Snapshot{}
	var cursor string
	for {
		remaining := opts.MaxIssues - len(snap.Issues)
		if remaining <= 0 {
			snap.Truncated = true
			break
		}
		page := min(opts.PageSize, remaining)

		raw, err := c.graphQL(ctx, owner, name, cursor, page)
		if err != nil {
			return nil, err
		}
		decoded, err := decode(raw)
		if err != nil {
			return nil, err
		}

		if snap.Repository.NameWithOwner == "" {
			snap.Repository = decoded.Repository
			snap.TotalOpen = decoded.TotalOpen
		}
		snap.Issues = append(snap.Issues, decoded.Issues...)

		if !decoded.HasNext || decoded.EndCursor == "" {
			break
		}
		cursor = decoded.EndCursor
	}

	if len(snap.Issues) == 0 {
		return snap, ErrNoIssues
	}
	return snap, nil
}

func (c *Client) graphQL(ctx context.Context, owner, name, cursor string, page int) ([]byte, error) {
	args := []string{
		"api", "graphql",
		"-f", "query=" + query,
		"-f", "owner=" + owner,
		"-f", "name=" + name,
		"-F", fmt.Sprintf("page=%d", page),
	}
	if cursor != "" {
		args = append(args, "-f", "cursor="+cursor)
	}
	return c.run(ctx, nil, args...)
}

func (c *Client) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	if c.Run != nil {
		return c.Run(ctx, stdin, args...)
	}
	bin := c.GH
	if bin == "" {
		bin = "gh"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ghdata: gh: %w: %s", err, trim(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// query asks for open issues only. The label list is fetched, never filtered on.
const query = `query($owner: String!, $name: String!, $cursor: String, $page: Int!) {
  repository(owner: $owner, name: $name) {
    nameWithOwner
    diskUsage
    primaryLanguage { name }
    defaultBranchRef { name target { ... on Commit { oid } } }
    issues(states: OPEN, first: $page, after: $cursor, orderBy: {field: UPDATED_AT, direction: DESC}) {
      totalCount
      pageInfo { hasNextPage endCursor }
      nodes {
        number title body url createdAt updatedAt state stateReason
        author { login }
        authorAssociation
        assignees(first: 5) { nodes { login } }
        labels(first: 20) { nodes { name } }
        comments(first: 10) {
          totalCount
          nodes { author { login } body createdAt authorAssociation }
        }
        timelineItems(itemTypes: [CROSS_REFERENCED_EVENT, CONNECTED_EVENT], first: 10) {
          nodes {
            __typename
            ... on CrossReferencedEvent { source { __typename ... on PullRequest { number state url merged } } }
            ... on ConnectedEvent { subject { __typename ... on PullRequest { number state url merged } } }
          }
        }
      }
    }
  }
}`

type authorNode struct {
	Login string `json:"login"`
}

type pullRequestNode struct {
	Typename string `json:"__typename"`
	Source   *struct {
		Typename string `json:"__typename"`
		Number   int    `json:"number"`
		State    string `json:"state"`
		URL      string `json:"url"`
		Merged   bool   `json:"merged"`
	} `json:"source"`
	Subject *struct {
		Typename string `json:"__typename"`
		Number   int    `json:"number"`
		State    string `json:"state"`
		URL      string `json:"url"`
		Merged   bool   `json:"merged"`
	} `json:"subject"`
}

type issueNode struct {
	Number            int         `json:"number"`
	Title             string      `json:"title"`
	Body              string      `json:"body"`
	URL               string      `json:"url"`
	State             string      `json:"state"`
	StateReason       string      `json:"stateReason"`
	AuthorAssociation string      `json:"authorAssociation"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
	Author            *authorNode `json:"author"`
	Assignees         struct {
		Nodes []authorNode `json:"nodes"`
	} `json:"assignees"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Comments struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Author            *authorNode `json:"author"`
			Body              string      `json:"body"`
			CreatedAt         time.Time   `json:"createdAt"`
			AuthorAssociation string      `json:"authorAssociation"`
		} `json:"nodes"`
	} `json:"comments"`
	TimelineItems struct {
		Nodes []pullRequestNode `json:"nodes"`
	} `json:"timelineItems"`
}

type decodeTarget struct {
	Repository
	TotalOpen int
	Issues    []Issue
	HasNext   bool
	EndCursor string
}

func decode(raw []byte) (*decodeTarget, error) {
	var payload struct {
		Data struct {
			Repository struct {
				NameWithOwner   string `json:"nameWithOwner"`
				DiskUsage       int    `json:"diskUsage"`
				PrimaryLanguage *struct {
					Name string `json:"name"`
				} `json:"primaryLanguage"`
				DefaultBranchRef *struct {
					Name   string `json:"name"`
					Target *struct {
						Oid string `json:"oid"`
					} `json:"target"`
				} `json:"defaultBranchRef"`
				Issues struct {
					TotalCount int `json:"totalCount"`
					PageInfo   struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []issueNode `json:"nodes"`
				} `json:"issues"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("ghdata: decode: %w", err)
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("ghdata: graphql: %s", payload.Errors[0].Message)
	}

	r := payload.Data.Repository
	out := &decodeTarget{
		Repository: Repository{NameWithOwner: r.NameWithOwner, DiskUsageKB: r.DiskUsage},
		TotalOpen:  r.Issues.TotalCount,
		HasNext:    r.Issues.PageInfo.HasNextPage,
		EndCursor:  r.Issues.PageInfo.EndCursor,
	}
	if r.PrimaryLanguage != nil {
		out.PrimaryLanguage = r.PrimaryLanguage.Name
	}
	if r.DefaultBranchRef != nil {
		out.DefaultBranch = r.DefaultBranchRef.Name
		if r.DefaultBranchRef.Target != nil {
			out.CommitSHA = r.DefaultBranchRef.Target.Oid
		}
	}

	for _, n := range r.Issues.Nodes {
		out.Issues = append(out.Issues, convertIssue(n))
	}
	return out, nil
}

func convertIssue(n issueNode) Issue {
	iss := Issue{
		Number:            n.Number,
		Title:             n.Title,
		Body:              n.Body,
		URL:               n.URL,
		State:             n.State,
		StateReason:       n.StateReason,
		AuthorAssociation: n.AuthorAssociation,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
		CommentCount:      n.Comments.TotalCount,
	}
	if n.Author != nil {
		iss.Author = n.Author.Login
	}
	for _, a := range n.Assignees.Nodes {
		iss.Assignees = append(iss.Assignees, a.Login)
	}
	for _, l := range n.Labels.Nodes {
		iss.Labels = append(iss.Labels, l.Name)
	}
	for _, c := range n.Comments.Nodes {
		comment := Comment{Body: c.Body, AuthorAssociation: c.AuthorAssociation, CreatedAt: c.CreatedAt}
		if c.Author != nil {
			comment.Author = c.Author.Login
		}
		iss.Comments = append(iss.Comments, comment)
	}
	for _, t := range n.TimelineItems.Nodes {
		if pr := convertPR(t); pr != nil {
			iss.LinkedPRs = append(iss.LinkedPRs, *pr)
		}
	}
	return iss
}

func convertPR(n pullRequestNode) *PullRequest {
	switch {
	case n.Source != nil && n.Source.Typename == "PullRequest":
		return &PullRequest{Number: n.Source.Number, State: n.Source.State, URL: n.Source.URL, Merged: n.Source.Merged}
	case n.Subject != nil && n.Subject.Typename == "PullRequest":
		return &PullRequest{Number: n.Subject.Number, State: n.Subject.State, URL: n.Subject.URL, Merged: n.Subject.Merged}
	default:
		return nil
	}
}

func trim(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}
