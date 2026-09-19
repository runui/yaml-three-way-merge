package smdmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOverrideDocumentsPreservesResetAndWrites(t *testing.T) {
	documents, err := parseOverrideDocuments([]byte(`services:
  app:
    ports: !reset []
---
services:
  app:
    ports:
      - 8080:80
`))
	require.NoError(t, err)
	require.Len(t, documents, 2)
	assert.Equal(t, [][]string{{"services", "app", "ports"}}, documents[0].resets)
	assert.NotContains(t, string(documents[0].yaml), "ports")
	assert.Empty(t, documents[1].resets)
	assert.Contains(t, string(documents[1].yaml), "8080:80")
}
