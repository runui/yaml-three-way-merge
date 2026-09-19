package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/runui/yaml-three-way-merge/internal/validation"
	"github.com/runui/yaml-three-way-merge/merge"
)

type intentTotals struct {
	UserAdded           int                   `json:"user_added"`
	UserModified        int                   `json:"user_modified"`
	UserRemoved         int                   `json:"user_removed"`
	UpstreamAdded       int                   `json:"upstream_added"`
	UpstreamModified    int                   `json:"upstream_modified"`
	UpstreamRemoved     int                   `json:"upstream_removed"`
	IndependentUser     int                   `json:"independent_user"`
	IndependentUpstream int                   `json:"independent_upstream"`
	OriginRewrites      int                   `json:"origin_rewrites"`
	Conflicts           merge.ConflictSummary `json:"conflicts"`
}

type summary struct {
	validation.MergeSummary
	Intent intentTotals `json:"intent"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate-smd-intent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("fixtures", "fixtures", "fixture directory")
	jsonOutput := flags.Bool("json", false, "write JSON summary")
	caseID := flags.String("case", "", "run one fixture case")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cases, _, err := corpus.Load(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load fixtures: %v\n", err)
		return 2
	}
	var result summary
	for _, item := range cases {
		if *caseID != "" && item.Metadata.ID != *caseID {
			continue
		}
		result.Total++
		merged, err := merge.Merge(merge.Input{PreviousBase: item.BaseOld, UserOverride: item.User, TargetBase: item.BaseNew})
		if err != nil {
			result.RecordError(item.Metadata.ID, item.Metadata.Field, err, 20)
			continue
		}
		accumulateIntent(&result.Intent, merged.Report)
		equal, err := merge.Equivalent(merged.YAML, item.Expected)
		if err != nil {
			result.RecordError(item.Metadata.ID, item.Metadata.Field, fmt.Errorf("compare: %w", err), 20)
			continue
		}
		result.RecordMatch(item.Metadata.ID, item.Metadata.Field, equal)
		if !equal && *caseID != "" {
			_, _ = fmt.Fprintf(stderr, "actual:\n%s\nexpected:\n%s\nreport:\n%+v\n", merged.YAML, item.Expected, merged.Report)
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
		result.WriteText(stdout, false)
	}
	return result.ExitCode()
}

func accumulateIntent(total *intentTotals, report merge.Report) {
	total.UserAdded += report.User.Added
	total.UserModified += report.User.Modified
	total.UserRemoved += report.User.Removed
	total.UpstreamAdded += report.Upstream.Added
	total.UpstreamModified += report.Upstream.Modified
	total.UpstreamRemoved += report.Upstream.Removed
	total.IndependentUser += report.IndependentUser
	total.IndependentUpstream += report.IndependentUpstream
	total.OriginRewrites += report.OriginRewrites
	total.Conflicts.WriteWrite += report.Conflicts.WriteWrite
	total.Conflicts.WriteRemove += report.Conflicts.WriteRemove
	total.Conflicts.RemoveWrite += report.Conflicts.RemoveWrite
	total.Conflicts.BothRemove += report.Conflicts.BothRemove
}
