package validation

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/internal/compose"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixtureCorpusAgainstCurrentImplementation(t *testing.T) {
	cases, _, err := corpus.Load(filepath.Join("..", "..", "fixtures"))
	require.NoError(t, err)
	for _, item := range cases {
		t.Run(item.Metadata.ID, func(t *testing.T) {
			result := Run(item)
			require.NoError(t, result.Error)
			assert.True(t, result.Idempotent, "new override is not idempotent\nactual:\n%s\noverride:\n%s", result.Actual, result.Override)
			assert.True(t, result.Matches, "user-intent mismatch\nactual:\n%s\nexpected:\n%s\nuser override:\n%s\nrebased override:\n%s", result.Actual, item.Expected, item.User, result.Override)
		})
	}
}

func TestFixtureUserOverridesDoNotResetWholeServices(t *testing.T) {
	cases, _, err := corpus.Load(filepath.Join("..", "..", "fixtures"))
	require.NoError(t, err)
	for _, item := range cases {
		assert.NotContains(t, string(item.User), "services: !reset {}", item.Metadata.ID)
		assert.False(t, strings.Contains(string(item.User), "services:\n  !reset"), item.Metadata.ID)
	}
}

func TestFixtureManifestIsComplete(t *testing.T) {
	cases, manifest, err := corpus.Load(filepath.Join("..", "..", "fixtures"))
	require.NoError(t, err)
	assert.Equal(t, len(cases), manifest.CaseCount)
	assert.Equal(t, 16132, manifest.CaseCount)
	assert.Equal(t, 15, manifest.StateCount)
	assert.Equal(t, len(corpus.ArrayFields()), len(manifest.Fields))
	for _, field := range manifest.Fields {
		assert.Equal(t, 15, field.MatrixCases, field.ID)
		if field.Class != corpus.ClassAtomic {
			assert.Equal(t, 225, field.CrossCases, field.ID)
		}
		for _, spec := range corpus.ArrayFields() {
			if spec.ID == field.ID {
				assert.Equal(t, spec.PairSemantics, field.PairSemantics, field.ID)
			}
		}
	}
}

func TestArrayFieldRegistryIsAuditable(t *testing.T) {
	fields := corpus.ArrayFields()
	ids := make([]string, 0, len(fields))
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		ids = append(ids, field.ID)
		paths = append(paths, field.MetadataPath())
	}
	assert.Len(t, ids, 70)
	assert.Equal(t, len(ids), len(unique(ids)), "duplicate field IDs")
	assert.Equal(t, len(paths), len(unique(paths)), "duplicate YAML paths")
	for _, required := range []string{
		"service.ports", "service.volumes", "service.devices", "service.configs", "service.secrets",
		"service.environment", // map-like fields are intentionally excluded from this array-focused phase.
	} {
		if required == "service.environment" {
			assert.False(t, slices.Contains(ids, required), "environment is not an array corpus field")
			continue
		}
		assert.True(t, slices.Contains(ids, required), required)
	}
}

func TestGeneratedUserOverridesExpressDeclaredIntent(t *testing.T) {
	cases, _, err := corpus.Generate()
	require.NoError(t, err)
	for _, item := range cases {
		t.Run(item.Metadata.ID, func(t *testing.T) {
			actual, err := compose.MergeYAML(item.BaseOld, item.User)
			require.NoError(t, err)
			equal, err := compose.SemanticYAMLEqual(actual, item.UserEffective)
			require.NoError(t, err)
			assert.True(t, equal, "user.yml does not reproduce declared user intent\nactual:\n%s\nintent:\n%s\nuser override:\n%s", actual, item.UserEffective, item.User)
		})
	}
}

func unique(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
