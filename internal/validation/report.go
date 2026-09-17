package validation

import (
	"encoding/json"
	"fmt"
	"io"
)

func WriteJSON(writer io.Writer, results []Result) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func WriteMarkdown(writer io.Writer, results []Result) error {
	if _, err := fmt.Fprintln(writer, "| Case | Field | Current expected | Current | SMD expected | SMD | Round trip | Idempotent |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "| --- | --- | --- | --- | --- | --- | --- | --- |"); err != nil {
		return err
	}
	currentMatched := 0
	currentKnownGaps := 0
	smdMatched := 0
	smdKnownGaps := 0
	unexpected := 0
	for _, result := range results {
		verdict := "MATCH"
		if !result.MatchesDesired {
			verdict = "DIFF"
		}
		if result.Error != "" {
			verdict = "ERROR: " + result.Error
		}
		smdVerdict := "MATCH"
		if !result.SMDMatchesDesired {
			smdVerdict = "DIFF"
		}
		if result.SMDError != "" {
			smdVerdict = "ERROR: " + result.SMDError
		}
		if result.MatchesClassification {
			switch result.Case.Expectation {
			case ExpectationMatch:
				currentMatched++
			case ExpectationKnownGap:
				currentKnownGaps++
			}
		} else {
			unexpected++
		}
		if result.SMDMatchesClassification {
			switch result.Case.ExpectedSMDClassification() {
			case ExpectationMatch:
				smdMatched++
			case ExpectationKnownGap:
				smdKnownGaps++
			}
		} else {
			unexpected++
		}
		if _, err := fmt.Fprintf(writer, "| `%s` | `%s` | %s | %s | %s | %s | %t | %t |\n",
			result.Case.ID, result.Case.Field, result.Case.Expectation, verdict,
			result.Case.ExpectedSMDClassification(), smdVerdict, result.ReconstructionOK, result.Idempotent); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer,
		"\nSummary: current=%d matches/%d known gaps; SMD=%d matches/%d known gaps; %d unexpected classification changes.\n",
		currentMatched, currentKnownGaps, smdMatched, smdKnownGaps, unexpected)
	return err
}
