package validation

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/runui/yaml-three-way-merge/internal/compose"
	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/runui/yaml-three-way-merge/internal/smdintent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixtureCorpusAgainstIntentImplementation is the regression gate for the
// replayed user-intent merge: every fixture must reproduce the independently
// generated expected result. The cases in knownUnsupportedIntent are excluded
// because their expected result is not determined by the three input documents.
func TestFixtureCorpusAgainstIntentImplementation(t *testing.T) {
	cases, _, err := corpus.Load(filepath.Join("..", "..", "fixtures"))
	require.NoError(t, err)
	var unexpected []string
	for _, item := range cases {
		merged, err := smdintent.Merge(item.BaseOld, item.User, item.BaseNew)
		require.NoError(t, err, item.Metadata.ID)
		equal, err := smdintent.Equivalent(merged.YAML, item.Expected)
		require.NoError(t, err, item.Metadata.ID)
		if equal {
			continue
		}
		if _, known := knownUnsupportedIntent[item.Metadata.ID]; known {
			continue
		}
		unexpected = append(unexpected, item.Metadata.ID)
	}
	assert.Empty(t, unexpected, "unexpected merge mismatches")
}

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
	assert.Equal(t, 15, manifest.StateCount)
	fields := corpus.ArrayFields()
	assert.Equal(t, len(fields), len(manifest.Fields))
	// Seven explicit port acceptance scenarios are appended in Generate.
	expected := 7
	for _, field := range manifest.Fields {
		assert.Equal(t, 15, field.MatrixCases, field.ID)
		if field.Class != corpus.ClassAtomic {
			assert.Equal(t, 225, field.CrossCases, field.ID)
			expected += 15 + 225
		} else {
			expected += 15
		}
		for _, spec := range fields {
			if spec.ID == field.ID {
				assert.Equal(t, spec.PairSemantics, field.PairSemantics, field.ID)
			}
		}
	}
	assert.Equal(t, expected, manifest.CaseCount)
}

func TestArrayFieldRegistryIsAuditable(t *testing.T) {
	fields := corpus.ArrayFields()
	ids := make([]string, 0, len(fields))
	var baseIDs, basePaths []string
	for _, field := range fields {
		ids = append(ids, field.ID)
		if field.Variant == "" {
			baseIDs = append(baseIDs, field.ID)
			basePaths = append(basePaths, field.MetadataPath())
		}
	}
	assert.Equal(t, len(ids), len(unique(ids)), "duplicate field IDs")
	assert.Equal(t, len(baseIDs), len(unique(baseIDs)), "duplicate base field IDs")
	assert.Equal(t, len(basePaths), len(unique(basePaths)), "duplicate base YAML paths")
	assert.Len(t, baseIDs, 70)
	for _, required := range []string{
		"service.ports", "service.volumes", "service.devices", "service.configs", "service.secrets",
		"service.environment", // map-like fields are intentionally excluded from this array-focused phase.
	} {
		if required == "service.environment" {
			assert.False(t, slices.Contains(baseIDs, required), "environment is not an array corpus field")
			continue
		}
		assert.True(t, slices.Contains(baseIDs, required), required)
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
