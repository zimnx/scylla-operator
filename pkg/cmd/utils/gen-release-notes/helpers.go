// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"
)

const (
	repositoryOwner = "kubernetes"
	repositoryName  = "kubernetes"
)

func githubClient(ctx context.Context) *github.Client {
	var httpClient *http.Client
	if os.Getenv("GH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GH_TOKEN")},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	return github.NewClient(httpClient)
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
