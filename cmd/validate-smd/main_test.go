package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSMDCorpusBaseline runs cmd/validate-smd over the whole fixture
// corpus without the project comparison and locks the documented baseline.
func TestValidateSMDCorpusBaseline(t *testing.T) {
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
	assert.Equal(t, 27573, got.Matched)
	assert.Equal(t, 1057, got.Mismatched)
	assert.Zero(t, got.Errors)
}

// TestValidateSMDCase runs a single fixture case, which is decided by the direct
// user-delta strategy and therefore exits 0.
func TestValidateSMDCase(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-fixtures", filepath.Join("..", "..", "fixtures"),
		"-case", "atomic/healthcheck.test/single/06-all-unchanged",
	}, &stdout, &stderr)
	assert.Zero(t, code, "stderr: %s", stderr.String())
	assert.Contains(t, stdout.String(), "Total: 1")
	assert.Contains(t, stdout.String(), "Matched: 1")
}

func TestValidateSMDCaseNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-fixtures", filepath.Join("..", "..", "fixtures"),
		"-case", "does/not/exist",
	}, &stdout, &stderr)
	assert.Equal(t, 2, code)
}
