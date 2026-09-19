package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSMDIntentCorpusBaseline runs cmd/validate-smd-intent over the whole
// fixture corpus and locks the documented baseline. The residual 50 mismatches
// are the known-unobservable cases; the command exits 1 while they remain.
func TestValidateSMDIntentCorpusBaseline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-fixtures", filepath.Join("..", "..", "fixtures"), "-json"}, &stdout, &stderr)
	require.Equal(t, 1, code, "stderr: %s", stderr.String())

	var got struct {
		Total      int `json:"total"`
		Matched    int `json:"matched"`
		Mismatched int `json:"mismatched"`
		Errors     int `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout: %s", stdout.String())
	assert.Equal(t, 28630, got.Total)
	assert.Equal(t, 27430, got.Matched)
	assert.Equal(t, 1200, got.Mismatched)
	assert.Zero(t, got.Errors)
}

// TestValidateSMDIntentCase runs a single fixture case that the two-delta
// strategy decides, so the command exits 0.
func TestValidateSMDIntentCase(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-fixtures", filepath.Join("..", "..", "fixtures"),
		"-case", "atomic/healthcheck.test/single/06-all-unchanged",
	}, &stdout, &stderr)
	assert.Zero(t, code, "stderr: %s", stderr.String())
	assert.Contains(t, stdout.String(), "Total: 1")
	assert.Contains(t, stdout.String(), "Matched: 1")
}

func TestValidateSMDIntentCaseNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-fixtures", filepath.Join("..", "..", "fixtures"),
		"-case", "does/not/exist",
	}, &stdout, &stderr)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr.String(), "case not found")
}
