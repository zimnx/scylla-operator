// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

func formatPR(maintainers []string) func(pr *github.PullRequest) string {
	return func(pr *github.PullRequest) string {
		suffix := fmt.Sprintf("[#%d](%s)", pr.GetNumber(), pr.GetHTMLURL())
		if !isMaintainer(maintainers, pr.User) {
			suffix += fmt.Sprintf(",[@%s](%s)", pr.GetUser().GetLogin(), pr.GetUser().GetHTMLURL())
		}
		return fmt.Sprintf("* %s (%s)", pr.GetTitle(), suffix)
	}
}

func isMaintainer(maintainers []string, user *github.User) bool {
	for _, m := range maintainers {
		if user.GetLogin() == m {
			return true
		}
	}

	return false
}

func uncategorized(prs []*github.PullRequest) []*github.PullRequest {
	var filtered []*github.PullRequest
	for _, pr := range prs {
		if len(pr.Labels) == 0 {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}

func filterByLabelKinds(prs []*github.PullRequest, kinds []string) []*github.PullRequest {
	var filtered []*github.PullRequest
	for _, pr := range prs {
		for _, label := range pr.Labels {
			parts := strings.Split(label.GetName(), "/")
			if len(parts) == 2 && parts[0] == "kind" {
				kind := parts[1]
				for _, k := range kinds {
					if k == kind {
						filtered = append(filtered, pr)
					}
				}
			}
		}
	}
	return filtered
}

const (
	repositoryOwner = "scylladb"
	repositoryName  = "scylla-operator"
)

func listPullRequests(ctx context.Context, base, state string, pred func(request *github.PullRequest) bool) ([]*github.PullRequest, error) {
	var httpClient *http.Client
	if os.Getenv("GH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GH_TOKEN")},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	client := github.NewClient(httpClient)

	var filtered []*github.PullRequest

	page := 0
	for {
		prs, resp, err := client.PullRequests.List(ctx, repositoryOwner, repositoryName, &github.PullRequestListOptions{
			Base:        base,
			State:       state,
			ListOptions: github.ListOptions{Page: page},
		})
		if err != nil {
			return nil, errors.Wrap(err, "list pull requests")
		}

		for _, pr := range prs {
			if pred(pr) {
				filtered = append(filtered, pr)
			}
		}

		page = resp.NextPage
		if page == 0 {
			break
		}
	}

	return filtered, nil
}

type TimeFlag struct {
	time.Time
}

func (ct TimeFlag) String() string {
	return ct.Time.Format(time.RFC3339)
}

func (ct *TimeFlag) Set(v string) error {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return err
	}

	ct.Time = t
	return nil
}
func (ct TimeFlag) Type() string {
	return "string"
}
