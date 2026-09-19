package smdmerge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
