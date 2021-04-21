// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"errors"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
)

type GenerateOptions struct {
	genericclioptions.IOStreams

	OrganizationName string
	RepositoryName   string
	RepositoryPath   string

	Release string
	TagFrom string
	TagTo   string
}

func (o GenerateOptions) Validate() error {
	if o.Release == "" {
		return errors.New("release cannot be empty")
	}

	return nil
}

func NewGitGenerateOptions(streams genericclioptions.IOStreams) *GenerateOptions {
	return &GenerateOptions{
		IOStreams:        streams,
		OrganizationName: "scylladb",
		RepositoryName:   "scylla-operator",
		RepositoryPath:   ".",
	}
}
