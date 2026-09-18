package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateProjectCorpusBaseline runs cmd/validate over the whole fixture
// corpus and locks the documented baseline. The production
// RebaseRepositoryUpdate algorithm does not reproduce every user intent yet, so
// the command exits 1 and mismatched stays non-zero until it does.
func TestValidateProjectCorpusBaseline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-fixtures", filepath.Join("..", "..", "fixtures"), "-json"}, &stdout, &stderr)
	require.Equal(t, 1, code, "stderr: %s", stderr.String())

	var got struct {
		Total         int `json:"total"`
		Matched       int `json:"matched"`
		Mismatched    int `json:"mismatched"`
		Errors        int `json:"errors"`
		NonIdempotent int `json:"non_idempotent"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout: %s", stdout.String())
	assert.Equal(t, 20977, got.Total)
	assert.Equal(t, 17200, got.Matched)
	assert.Equal(t, 3777, got.Mismatched)
	assert.Zero(t, got.Errors)
	assert.Zero(t, got.NonIdempotent)
}

func TestValidateProjectLoadFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-fixtures", filepath.Join(t.TempDir(), "missing")}, &stdout, &stderr)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr.String(), "load fixtures")
}
