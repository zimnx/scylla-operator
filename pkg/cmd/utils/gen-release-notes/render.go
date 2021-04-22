// Copyright (C) 2021 ScyllaDB

package gen_release_notes

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/google/go-github/v35/github"
)

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

{{- with $uncategorized := ( Uncategorized .PRs ) }}
### Uncategorized

  {{- range $pr := $uncategorized }}
{{ FormatPR $pr }}
  {{- end }}
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

func renderReleaseNotes(o GenerateCommonOptions, pullRequests []*github.PullRequest) error {
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
