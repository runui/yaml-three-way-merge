package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/smdintent"
)

type intentTotals struct {
	UserAdded           int                       `json:"user_added"`
	UserModified        int                       `json:"user_modified"`
	UserRemoved         int                       `json:"user_removed"`
	UpstreamAdded       int                       `json:"upstream_added"`
	UpstreamModified    int                       `json:"upstream_modified"`
	UpstreamRemoved     int                       `json:"upstream_removed"`
	IndependentUser     int                       `json:"independent_user"`
	IndependentUpstream int                       `json:"independent_upstream"`
	OriginRewrites      int                       `json:"origin_rewrites"`
	Conflicts           smdintent.ConflictSummary `json:"conflicts"`
}

type summary struct {
	Total           int            `json:"total"`
	Matched         int            `json:"matched"`
	Mismatched      int            `json:"mismatched"`
	Errors          int            `json:"errors"`
	ByField         map[string]int `json:"mismatched_by_field,omitempty"`
	ErrorsByField   map[string]int `json:"errors_by_field,omitempty"`
	MismatchSamples []string       `json:"mismatch_samples,omitempty"`
	ErrorSamples    []string       `json:"error_samples,omitempty"`
	Intent          intentTotals   `json:"intent"`
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
	result := summary{ByField: map[string]int{}, ErrorsByField: map[string]int{}}
	found := false
	for _, item := range cases {
		if *caseID != "" && item.Metadata.ID != *caseID {
			continue
		}
		found = true
		result.Total++
		merged, err := smdintent.Merge(item.BaseOld, item.User, item.BaseNew)
		if err != nil {
			result.Errors++
			result.ErrorsByField[item.Metadata.Field]++
			if len(result.ErrorSamples) < 20 {
				result.ErrorSamples = append(result.ErrorSamples, item.Metadata.ID+": "+err.Error())
			}
			continue
		}
		accumulateIntent(&result.Intent, merged.Report)
		equal, err := smdintent.Equivalent(merged.YAML, item.Expected)
		if err != nil {
			result.Errors++
			result.ErrorsByField[item.Metadata.Field]++
			if len(result.ErrorSamples) < 20 {
				result.ErrorSamples = append(result.ErrorSamples, item.Metadata.ID+": compare: "+err.Error())
			}
			continue
		}
		if equal {
			result.Matched++
		} else {
			result.Mismatched++
			result.ByField[item.Metadata.Field]++
			if len(result.MismatchSamples) < 20 {
				result.MismatchSamples = append(result.MismatchSamples, item.Metadata.ID)
			}
			if *caseID != "" {
				_, _ = fmt.Fprintf(stderr, "actual:\n%s\nexpected:\n%s\nreport:\n%+v\n", merged.YAML, item.Expected, merged.Report)
			}
		}
	}
	if *caseID != "" && !found {
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
		fmt.Fprintf(stdout, "Total: %d\nMatched: %d\nMismatched: %d\nErrors: %d\n", result.Total, result.Matched, result.Mismatched, result.Errors)
		fields := make([]string, 0, len(result.ByField))
		for field := range result.ByField {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			fmt.Fprintf(stdout, "  %s: %d\n", field, result.ByField[field])
		}
	}
	if result.Errors > 0 || result.Mismatched > 0 {
		return 1
	}
	return 0
}

func accumulateIntent(total *intentTotals, report smdintent.Report) {
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
