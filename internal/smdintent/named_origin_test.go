package smdintent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runui/yaml-three-way-merge/internal/smdmerge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeNamedOriginsMatchesStableTokensInPair(t *testing.T) {
	old := []namedEntry{{name: "base-net-a"}, {name: "base-net-b"}}
	user := []namedEntry{{name: "user-net-a"}, {name: "base-net-b"}}
	upstream := []namedEntry{{name: "remote-net-a"}, {name: "base-net-b"}}
	merged, changed := mergeNamedOrigins(old, user, upstream)
	require.True(t, changed)
	require.Len(t, merged, 2)
	assert.Equal(t, "user-net-a", merged[0].name)
	assert.Equal(t, "base-net-b", merged[1].name)
}

func TestReconcileNamedOriginsWithFixturePrediction(t *testing.T) {
	dir := filepath.Join("..", "..", "fixtures", "named-item", "service.networks", "pair", "12-both-modify-different-user-wins__06-all-unchanged")
	oldYAML, err := os.ReadFile(filepath.Join(dir, "oldbase.yml"))
	require.NoError(t, err)
	userOverride, err := os.ReadFile(filepath.Join(dir, "user.yml"))
	require.NoError(t, err)
	newYAML, err := os.ReadFile(filepath.Join(dir, "newbase.yml"))
	require.NoError(t, err)
	scenario, err := smdmerge.CompileTypedScenario(oldYAML, userOverride, newYAML)
	require.NoError(t, err)
	userYAML, err := smdmerge.MarshalTypedValue(scenario.UserPrediction)
	require.NoError(t, err)
	content, rewrites, err := reconcileNamedOrigins(oldYAML, userYAML, newYAML, newYAML)
	require.NoError(t, err)
	assert.Equal(t, 1, rewrites, "user prediction:\n%s", userYAML)
	equal, err := Equivalent(content, []byte("services:\n  app:\n    image: busybox:latest\n    networks: [user-net-a, base-net-b]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s\nuser prediction:\n%s", content, userYAML)
}

func TestMergeReconcilesNamedOriginsFromFixture(t *testing.T) {
	dir := filepath.Join("..", "..", "fixtures", "named-item", "service.networks", "pair", "12-both-modify-different-user-wins__06-all-unchanged")
	oldYAML, err := os.ReadFile(filepath.Join(dir, "oldbase.yml"))
	require.NoError(t, err)
	userOverride, err := os.ReadFile(filepath.Join(dir, "user.yml"))
	require.NoError(t, err)
	newYAML, err := os.ReadFile(filepath.Join(dir, "newbase.yml"))
	require.NoError(t, err)
	result, err := Merge(oldYAML, userOverride, newYAML)
	require.NoError(t, err)
	scenario, err := smdmerge.CompileTypedScenario(oldYAML, userOverride, newYAML)
	require.NoError(t, err)
	userYAML, err := smdmerge.MarshalTypedValue(scenario.UserPrediction)
	require.NoError(t, err)
	reconciled, rewrites, err := reconcileNamedOrigins(oldYAML, userYAML, newYAML, result.YAML)
	require.NoError(t, err)
	assert.Equal(t, 1, rewrites, "user prediction:\n%s\nmerge result:\n%s\nreconciled:\n%s", userYAML, result.YAML, reconciled)
	assert.Equal(t, 1, result.Report.OriginRewrites, "actual:\n%s", result.YAML)
	equal, err := Equivalent(result.YAML, []byte("services:\n  app:\n    image: busybox:latest\n    networks: [user-net-a, base-net-b]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", result.YAML)
}

func TestReconcileNamedOriginsMatchesNormalizedPair(t *testing.T) {
	content, rewrites, err := reconcileNamedOrigins(
		[]byte("services:\n  app:\n    image: busybox\n    networks: [base-net-a, base-net-b]\n"),
		[]byte("services:\n  app:\n    image: busybox\n    networks:\n      user-net-a: {}\n      base-net-b: {}\n"),
		[]byte("services:\n  app:\n    image: busybox\n    networks: [remote-net-a, base-net-b]\n"),
		[]byte("services:\n  app:\n    image: busybox\n    networks:\n      remote-net-a: {}\n      user-net-a: {}\n      base-net-b: {}\n"),
	)
	require.NoError(t, err)
	assert.Equal(t, 1, rewrites)
	equal, err := Equivalent(content, []byte("services:\n  app:\n    image: busybox\n    networks: [user-net-a, base-net-b]\n"))
	require.NoError(t, err)
	assert.True(t, equal, "actual:\n%s", content)
}
