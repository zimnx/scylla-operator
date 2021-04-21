// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"context"
	"html/template"

	"github.com/google/go-github/v35/github"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_GEN_RELEASE_NOTES_"
)

func NewGenReleaseNotesCommand(ctx context.Context, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGenerateOptions(streams)

	cmd := &cobra.Command{
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

			if err := run(ctx, o); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.Release, "release", o.Release, "Name of the release.")
	cmd.Flags().StringVar(&o.BaseBranch, "base-branch", o.BaseBranch, "PRs based on provided branch will appear in output.")
	cmd.Flags().StringSliceVar(&o.Maintainers, "maintainers", o.Maintainers, "List of repository maintainers. These users won't be mentioned in release notes.")
	cmd.Flags().Var(&o.MergedFrom, "merged-from", "PRs before this timestamp won't be considered. Timestamp must be in RFC3339 format.")
	cmd.Flags().Var(&o.MergedTo, "merged-to", "PRs after this timestamp won't be considered. Timestamp must be in RFC3339 format.")

	return cmd
}

const releaseNotesTemplate = `
# Release notes for {{ .Release }}

## Changes By Kind

{{- range $prKind := .PRKinds }}
  {{- with $filteredPRs := ( FilterByLabelKinds $.PRs $prKind.Kinds ) }}

### {{ $prKind.Title }}

    {{- range $pr := $filteredPRs }}
{{ FormatPR $pr }}
    {{- end }}
  {{- end }}
{{- end }}

### Uncategorized

{{- range $pr := ( Uncategorized .PRs ) }}
{{ FormatPR $pr }}
{{- end }}

---
`

type prKind struct {
	Title string
	Kinds []string
}

type releaseNoteData struct {
	PRKinds []prKind
	PRs     []*github.PullRequest
	Release string
}

func run(ctx context.Context, o *GenerateOptions) error {
	pullRequests, err := listPullRequests(ctx, o.BaseBranch, "closed", func(pr *github.PullRequest) bool {
		return o.MergedFrom.Before(pr.GetMergedAt()) && o.MergedTo.After(pr.GetMergedAt())
	})
	if err != nil {
		return err
	}

	data := releaseNoteData{
		Release: o.Release,
		PRs:     pullRequests,
		PRKinds: []prKind{
			{Title: "API Changes", Kinds: []string{"api-change"}},
			{Title: "Features", Kinds: []string{"feature"}},
			{Title: "Bugs", Kinds: []string{"bug"}},
			{Title: "Documentation", Kinds: []string{"documentation"}},
			{Title: "Flaky/Failing Tests", Kinds: []string{"failing-test", "flake"}},
			{Title: "Other", Kinds: []string{"cleanup"}},
		},
	}

	t := template.New("release-notes").Funcs(map[string]interface{}{
		"FilterByLabelKinds": filterByLabelKinds,
		"Uncategorized":      uncategorized,
		"FormatPR":           formatPR(o.Maintainers),
	})
	t = template.Must(t.Parse(releaseNotesTemplate))
	if err := t.Execute(o.Out, data); err != nil {
		return err
	}

	return nil
}
