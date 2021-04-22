// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"context"

	"github.com/google/go-github/v35/github"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_GEN_RELEASE_NOTES_"
)

func NewGenGithubReleaseNotesCommand(ctx context.Context, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGithubGenerateOptions(streams)

	cmd := &cobra.Command{
		Use:   "github",
		Short: "Generates release notes based on merged GitHub PRs from provided time range.",
		Long: templates.LongDesc(`
		Scylla Operator Release Notes

		This command generates release notes based on merged PR from provided time range. 
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			ghClient := githubClient(ctx)
			pullRequests, err := listPullRequests(ctx, ghClient, o.BaseBranch, "closed", func(pr *github.PullRequest) bool {
				return o.MergedFrom.Before(pr.GetMergedAt()) && o.MergedTo.After(pr.GetMergedAt())
			})
			if err != nil {
				return err
			}

			if err := renderReleaseNotes(o.GenerateCommonOptions, pullRequests); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.Release, "release", o.Release, "Name of the release.")
	cmd.Flags().StringSliceVar(&o.Maintainers, "maintainers", o.Maintainers, "List of repository maintainers. These users won't be mentioned in release notes.")
	cmd.Flags().StringVar(&o.BaseBranch, "base-branch", o.BaseBranch, "PRs based on provided branch will appear in output.")
	cmd.Flags().Var(&o.MergedFrom, "merged-from", "PRs before this timestamp won't be considered. Timestamp must be in RFC3339 format.")
	cmd.Flags().Var(&o.MergedTo, "merged-to", "PRs after this timestamp won't be considered. Timestamp must be in RFC3339 format.")

	return cmd
}

func listPullRequests(ctx context.Context, ghClient *github.Client, base, state string, pred func(request *github.PullRequest) bool) ([]*github.PullRequest, error) {
	var filtered []*github.PullRequest
	page := 0
	for {
		prs, resp, err := ghClient.PullRequests.List(ctx, repositoryOwner, repositoryName, &github.PullRequestListOptions{
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
