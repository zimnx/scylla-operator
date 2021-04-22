// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/google/go-github/v35/github"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewGenGitReleaseNotesCommand(ctx context.Context, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGitGenerateOptions(streams)

	cmd := &cobra.Command{
		Use:   "git",
		Short: "Generates release notes based on merge commits from local git repository between two tags.",
		Long: templates.LongDesc(`
		Scylla Operator Release Notes

		This command generates release notes based on merge commits from local git repository between two tags.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			ghClient := githubClient(ctx)
			pullRequests, err := pullRequestsFromMergeCommits(ctx, ghClient, o.RepositoryPath, o.TagFrom, o.TagTo)
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
	cmd.Flags().StringVar(&o.RepositoryPath, "repository-path", o.RepositoryPath, "Path to git repository.")
	cmd.Flags().StringVar(&o.TagFrom, "tag-from", o.TagFrom, "PRs merged after before this tag won't be considered.")
	cmd.Flags().StringVar(&o.TagTo, "tag-to", o.TagTo, "PRs merged before before this tag won't be considered.")

	return cmd
}

func pullRequestsFromMergeCommits(ctx context.Context, ghClient *github.Client, repositoryPath, tagFrom, tagTo string) ([]*github.PullRequest, error) {
	repo, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, err
	}

	commits, err := listCommitsBetween(repo, tagFrom, tagTo)
	if err != nil {
		return nil, err
	}

	pullRequests := make([]*github.PullRequest, 0, len(commits))
	for _, c := range commits {
		if !isMergeCommit(c) {
			continue
		}

		n, err := parsePullRequestNumber(c.Message)
		if err != nil {
			if errors.Is(ErrNotAPullRequest, err) {
				continue
			}
			return nil, err
		}

		pr, _, err := ghClient.PullRequests.Get(ctx, repositoryOwner, repositoryName, n)
		if err != nil {
			return nil, err
		}
		pullRequests = append(pullRequests, pr)
	}

	return pullRequests, nil
}

func isMergeCommit(commit *object.Commit) bool {
	return commit.NumParents() > 1
}

var (
	pullRequestNumberRe = regexp.MustCompile(`^Merge pull request #(\d+) .*$`)
	ErrNotAPullRequest  = fmt.Errorf("commit is not a pull request")
)

// Merge commit originating from merged PR has following format:
// 'Merge pull request #<pr-number> from <author>/<branch>'
func parsePullRequestNumber(message string) (int, error) {
	firstLine := strings.Split(message, "\n")[0]
	m := pullRequestNumberRe.FindStringSubmatch(firstLine)
	if m == nil {
		return 0, ErrNotAPullRequest
	}

	return strconv.Atoi(m[1])
}

func commitFromTag(r *git.Repository, tag string) (*object.Commit, error) {
	ref, err := r.Tag(tag)
	if err != nil {
		return nil, fmt.Errorf("tag %w", err)
	}

	t, err := r.TagObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("tag object, %w", err)
	}

	commit, err := r.CommitObject(t.Target)
	if err != nil {
		return nil, fmt.Errorf("commit object, %w", err)
	}

	return commit, nil
}

func listCommitsBetween(r *git.Repository, tagFrom, tagTo string) ([]*object.Commit, error) {
	fromCommit, err := commitFromTag(r, tagFrom)
	if err != nil {
		return nil, err
	}

	toCommit, err := commitFromTag(r, tagTo)
	if err != nil {
		return nil, err
	}

	mergeBaseCommits, err := toCommit.MergeBase(fromCommit)
	if err != nil {
		return nil, fmt.Errorf("%w: merge base", err)
	}
	if len(mergeBaseCommits) == 0 {
		return nil, errors.New("common merge base not found")
	}

	commitIter, err := r.Log(&git.LogOptions{From: toCommit.Hash})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	var commitsBetweenTags []*object.Commit
	if err := commitIter.ForEach(func(commit *object.Commit) error {
		for _, mergeBaseCommit := range mergeBaseCommits {
			if mergeBaseCommit.Hash == commit.Hash {
				return storer.ErrStop
			}
		}

		commitsBetweenTags = append(commitsBetweenTags, commit)
		return nil
	}); err != nil {
		return nil, err
	}

	return commitsBetweenTags, nil
}
