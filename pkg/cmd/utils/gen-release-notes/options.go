// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"errors"
	"time"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
)

type GenerateOptions struct {
	genericclioptions.IOStreams

	BaseBranch string
	Release    string
	MergedFrom TimeFlag
	MergedTo   TimeFlag

	Maintainers []string
}

func (o GenerateOptions) Validate() error {
	if o.Release == "" {
		return errors.New("release cannot be empty")
	}

	return nil
}

func NewGenerateOptions(streams genericclioptions.IOStreams) *GenerateOptions {
	return &GenerateOptions{
		IOStreams:  streams,
		MergedTo:   TimeFlag{time.Now()},
		BaseBranch: "master",
	}
}
