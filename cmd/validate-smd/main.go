package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/smdmerge"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/validation"
)

type summary struct {
	Total              int            `json:"total"`
	Matched            int            `json:"matched"`
	Mismatched         int            `json:"mismatched"`
	Errors             int            `json:"errors"`
	ByField            map[string]int `json:"mismatched_by_field,omitempty"`
	ErrorsByField      map[string]int `json:"errors_by_field,omitempty"`
	ErrorSamples       []string       `json:"error_samples,omitempty"`
	MismatchSamples    []string       `json:"mismatch_samples,omitempty"`
	ProjectMatched     int            `json:"project_matched,omitempty"`
	BothMatched        int            `json:"both_matched,omitempty"`
	SMDOnly            int            `json:"smd_only,omitempty"`
	ProjectOnly        int            `json:"project_only,omitempty"`
	SMDOnlyByField     map[string]int `json:"smd_only_by_field,omitempty"`
	ProjectOnlyByField map[string]int `json:"project_only_by_field,omitempty"`
}

func main() {
	root := flag.String("fixtures", "fixtures", "fixture directory")
	jsonOutput := flag.Bool("json", false, "write JSON summary")
	caseID := flag.String("case", "", "run one fixture case")
	compareProject := flag.Bool("compare-project", false, "compare matches with the project rebase implementation")
	flag.Parse()

	cases, _, err := corpus.Load(*root)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load fixtures: %v\n", err)
		os.Exit(2)
	}
	result := summary{Total: len(cases), ByField: map[string]int{}, ErrorsByField: map[string]int{}, SMDOnlyByField: map[string]int{}, ProjectOnlyByField: map[string]int{}}
	for _, item := range cases {
		if *caseID != "" && item.Metadata.ID != *caseID {
			result.Total--
			continue
		}
		actual, err := smdmerge.Merge(item.BaseOld, item.User, item.BaseNew)
		if err != nil {
			result.Errors++
			result.ErrorsByField[item.Metadata.Field]++
			if len(result.ErrorSamples) < 10 {
				result.ErrorSamples = append(result.ErrorSamples, item.Metadata.ID+": merge: "+err.Error())
			}
			continue
		}
		equal, err := smdmerge.Equivalent(actual, item.Expected)
		if err != nil {
			result.Errors++
			result.ErrorsByField[item.Metadata.Field]++
			if len(result.ErrorSamples) < 10 {
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
				_, _ = fmt.Fprintf(os.Stderr, "actual:\n%s\nexpected:\n%s\n", actual, item.Expected)
			}
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

	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(2)
		}
	} else {
		fmt.Printf("Total: %d\nMatched: %d\nMismatched: %d\nErrors: %d\n", result.Total, result.Matched, result.Mismatched, result.Errors)
		fields := make([]string, 0, len(result.ByField))
		for field := range result.ByField {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		fmt.Println("Mismatched by field:")
		for _, field := range fields {
			fmt.Printf("  %s: %d\n", field, result.ByField[field])
		}
	}
	if result.Errors > 0 || result.Mismatched > 0 {
		os.Exit(1)
	}
}
