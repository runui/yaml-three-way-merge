package validation

import (
	"fmt"
	"io"
	"sort"
)

// MergeSummary records merge outcomes independently of the selected strategy.
type MergeSummary struct {
	Total           int            `json:"total"`
	Matched         int            `json:"matched"`
	Mismatched      int            `json:"mismatched"`
	Errors          int            `json:"errors"`
	ByField         map[string]int `json:"mismatched_by_field,omitempty"`
	ErrorsByField   map[string]int `json:"errors_by_field,omitempty"`
	MismatchSamples []string       `json:"mismatch_samples,omitempty"`
	ErrorSamples    []string       `json:"error_samples,omitempty"`
}

func (s *MergeSummary) RecordError(id, field string, err error, sampleLimit int) {
	s.Errors++
	if s.ErrorsByField == nil {
		s.ErrorsByField = map[string]int{}
	}
	s.ErrorsByField[field]++
	if len(s.ErrorSamples) < sampleLimit {
		s.ErrorSamples = append(s.ErrorSamples, id+": "+err.Error())
	}
}

func (s *MergeSummary) RecordMatch(id, field string, equal bool) {
	if equal {
		s.Matched++
		return
	}
	s.Mismatched++
	if s.ByField == nil {
		s.ByField = map[string]int{}
	}
	s.ByField[field]++
	if len(s.MismatchSamples) < 20 {
		s.MismatchSamples = append(s.MismatchSamples, id)
	}
}

func (s MergeSummary) WriteText(out io.Writer, fieldHeading bool) {
	fmt.Fprintf(out, "Total: %d\nMatched: %d\nMismatched: %d\nErrors: %d\n", s.Total, s.Matched, s.Mismatched, s.Errors)
	fields := make([]string, 0, len(s.ByField))
	for field := range s.ByField {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	if fieldHeading {
		fmt.Fprintln(out, "Mismatched by field:")
	}
	for _, field := range fields {
		fmt.Fprintf(out, "  %s: %d\n", field, s.ByField[field])
	}
}

func (s MergeSummary) ExitCode() int {
	if s.Errors > 0 || s.Mismatched > 0 {
		return 1
	}
	return 0
}
