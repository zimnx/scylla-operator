// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"errors"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
)

type GenerateCommonOptions struct {
	genericclioptions.IOStreams

	Release     string
	Maintainers []string
}

func (o GenerateCommonOptions) Validate() error {
	if o.Release == "" {
		return errors.New("release cannot be empty")
	}

	return nil
}

type GitGenerateOptions struct {
	GenerateCommonOptions

	RepositoryPath string
	TagFrom        string
	TagTo          string
}

func (o GitGenerateOptions) Validate() error {
	if err := o.GenerateCommonOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func NewGitGenerateOptions(streams genericclioptions.IOStreams) *GitGenerateOptions {
	return &GitGenerateOptions{
		GenerateCommonOptions: GenerateCommonOptions{
			IOStreams: streams,
		},
		RepositoryPath: ".",
	}
}

type GithubGenerateOptions struct {
	GenerateCommonOptions

	BaseBranch string
	MergedFrom TimeFlag
	MergedTo   TimeFlag
}

func (o GithubGenerateOptions) Validate() error {
	if err := o.GenerateCommonOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func NewGithubGenerateOptions(streams genericclioptions.IOStreams) *GithubGenerateOptions {
	return &GithubGenerateOptions{
		GenerateCommonOptions: GenerateCommonOptions{
			IOStreams: streams,
		},
		BaseBranch: "master",
	}
}
