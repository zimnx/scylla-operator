// Copyright (c) 2024 ScyllaDB.

package tests

import (
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	ginkgotest "github.com/scylladb/scylla-operator/pkg/test/ginkgo"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunCommand(streams genericclioptions.IOStreams, testSuites ginkgotest.TestSuites, userAgent string) *cobra.Command {
	o := NewTestFrameworkOptions(streams, testSuites, userAgent)

	validTestSuiteNames := slices.ConvertSlice(testSuites, func(ts *ginkgotest.TestSuite) string {
		return ts.Name
	})

	cmd := &cobra.Command{
		Use: "run SUITE_NAME",
		Long: templates.LongDesc(`
		Runs a test suite
		`),
		ValidArgs: validTestSuiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate(args)
			if err != nil {
				return err
			}

			err = o.Complete(args)
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd)

	return cmd
}
