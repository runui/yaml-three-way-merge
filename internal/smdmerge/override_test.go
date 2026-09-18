package smdmerge

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

func TestMergeAppliesOverrideThroughTypedRemoveAndMerge(t *testing.T) {
	actual, err := Merge(
		[]byte("services:\n  app:\n    image: busybox\n    ports: [8080:80]\n"),
		[]byte("services:\n  app:\n    ports: !reset []\n---\nservices:\n  app:\n    ports: [9090:80]\n"),
		[]byte("services:\n  app:\n    image: busybox:new\n    ports: [8080:80, 8081:81]\n"),
	)
	require.NoError(t, err)
	equal, err := Equivalent(actual, []byte("services:\n  app:\n    image: busybox:new\n    ports: [9090:80, 8081:81]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", actual)
}
