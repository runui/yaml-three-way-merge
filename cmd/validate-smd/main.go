package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/runui/yaml-three-way-merge/internal/smdmerge"
	"github.com/runui/yaml-three-way-merge/internal/validation"
)

type summary struct {
	validation.MergeSummary
	ProjectMatched     int            `json:"project_matched,omitempty"`
	BothMatched        int            `json:"both_matched,omitempty"`
	SMDOnly            int            `json:"smd_only,omitempty"`
	ProjectOnly        int            `json:"project_only,omitempty"`
	SMDOnlyByField     map[string]int `json:"smd_only_by_field,omitempty"`
	ProjectOnlyByField map[string]int `json:"project_only_by_field,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate-smd", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("fixtures", "fixtures", "fixture directory")
	jsonOutput := flags.Bool("json", false, "write JSON summary")
	caseID := flags.String("case", "", "run one fixture case")
	compareProject := flags.Bool("compare-project", false, "compare matches with the project rebase implementation")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cases, _, err := corpus.Load(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load fixtures: %v\n", err)
		return 2
	}
	result := summary{SMDOnlyByField: map[string]int{}, ProjectOnlyByField: map[string]int{}}
	for _, item := range cases {
		if *caseID != "" && item.Metadata.ID != *caseID {
			continue
		}
		result.Total++
		actual, err := smdmerge.Merge(item.BaseOld, item.User, item.BaseNew)
		if err != nil {
			result.RecordError(item.Metadata.ID, item.Metadata.Field, fmt.Errorf("merge: %w", err), 10)
			continue
		}
		equal, err := smdmerge.Equivalent(actual, item.Expected)
		if err != nil {
			result.RecordError(item.Metadata.ID, item.Metadata.Field, fmt.Errorf("compare: %w", err), 10)
			continue
		}
		result.RecordMatch(item.Metadata.ID, item.Metadata.Field, equal)
		if !equal && *caseID != "" {
			_, _ = fmt.Fprintf(stderr, "actual:\n%s\nexpected:\n%s\n", actual, item.Expected)
		}
		if *compareProject {
			project := validation.Run(item)
			projectMatched := project.Error == nil && project.Idempotent && project.Matches
			if projectMatched {
				result.ProjectMatched++
			}
			switch {
			case equal && projectMatched:
				result.BothMatched++
			case equal:
				result.SMDOnly++
				result.SMDOnlyByField[item.Metadata.Field]++
			case projectMatched:
				result.ProjectOnly++
				result.ProjectOnlyByField[item.Metadata.Field]++
			}
		}
	}

	if *caseID != "" && result.Total == 0 {
		_, _ = fmt.Fprintf(stderr, "case not found: %s\n", *caseID)
		return 2
	}

	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
			return 2
		}
	} else {
		result.WriteText(stdout, true)
	}
	return result.ExitCode()
}
