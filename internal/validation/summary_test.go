package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeSummaryRecordsOutcomesAndBoundsSamples(t *testing.T) {
	var summary MergeSummary
	assert.Zero(t, summary.ExitCode())
	summary.RecordMatch("matched", "image", true)
	for range 25 {
		summary.RecordMatch("mismatched", "ports", false)
		summary.RecordError("invalid", "volumes", errors.New("merge failed"), 10)
	}
	assert.Equal(t, 1, summary.Matched)
	assert.Equal(t, 25, summary.Mismatched)
	assert.Equal(t, 25, summary.Errors)
	assert.Equal(t, map[string]int{"ports": 25}, summary.ByField)
	assert.Equal(t, map[string]int{"volumes": 25}, summary.ErrorsByField)
	assert.Len(t, summary.MismatchSamples, 20)
	assert.Len(t, summary.ErrorSamples, 10)
	assert.Equal(t, "invalid: merge failed", summary.ErrorSamples[0])
	assert.Equal(t, 1, summary.ExitCode())
}

func TestMergeSummaryPreservesFlatJSONAndSortedText(t *testing.T) {
	report := struct {
		MergeSummary
		Extra int `json:"extra"`
	}{MergeSummary: MergeSummary{Total: 2}, Extra: 3}
	report.RecordMatch("b", "volumes", false)
	report.RecordMatch("a", "ports", false)
	content, err := json.Marshal(report)
	require.NoError(t, err)
	assert.JSONEq(t, `{"total":2,"matched":0,"mismatched":2,"errors":0,"mismatched_by_field":{"ports":1,"volumes":1},"mismatch_samples":["b","a"],"extra":3}`, string(content))
	var text bytes.Buffer
	report.WriteText(&text, true)
	assert.Equal(t, "Total: 2\nMatched: 0\nMismatched: 2\nErrors: 0\nMismatched by field:\n  ports: 1\n  volumes: 1\n", text.String())
}
