package validation

import (
	"encoding/json"
	"fmt"
	"io"
)

type Summary struct {
	Total         int `json:"total"`
	Matched       int `json:"matched"`
	Mismatched    int `json:"mismatched"`
	Errors        int `json:"errors"`
	NonIdempotent int `json:"non_idempotent"`
}

func Summarize(results []Result) Summary {
	result := Summary{Total: len(results)}
	for _, item := range results {
		switch {
		case item.Error != nil:
			result.Errors++
		case !item.Idempotent:
			result.NonIdempotent++
		case item.Matches:
			result.Matched++
		default:
			result.Mismatched++
		}
	}
	return result
}

func WriteSummary(writer io.Writer, results []Result, jsonOutput bool) error {
	summary := Summarize(results)
	if jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	}
	_, err := fmt.Fprintf(writer, "Total: %d\nMatched: %d\nMismatched: %d\nErrors: %d\nNon-idempotent: %d\n", summary.Total, summary.Matched, summary.Mismatched, summary.Errors, summary.NonIdempotent)
	return err
}
