package main

import (
	"testing"

	"github.com/runui/yaml-three-way-merge/internal/compose"
	"github.com/stretchr/testify/require"
)

const testFixtureID = "atomic/service.image/single/12-both-modify-different-user-wins"

func testService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService("../../fixtures")
	require.NoError(t, err)
	return service
}

func TestMergeAlgorithmsProduceReproducibleOverrides(t *testing.T) {
	service := testService(t)
	fixture, ok := service.Fixture(testFixtureID)
	require.True(t, ok)

	for _, algorithm := range []string{"intent", "direct", "rebase"} {
		t.Run(algorithm, func(t *testing.T) {
			result, err := service.Merge(MergeRequest{
				Algorithm: algorithm, PreviousBase: fixture.PreviousBase,
				UserOverride: fixture.UserOverride, TargetBase: fixture.TargetBase,
				Expected: fixture.Expected, HasExpected: true,
			})
			require.NoError(t, err)
			require.NotEmpty(t, result.PreviousEffective)
			require.NotEmpty(t, result.NewEffective)
			require.NotNil(t, result.MatchesExpected)

			reconstructed, err := compose.MergeYAML([]byte(fixture.TargetBase), []byte(result.NewOverride))
			require.NoError(t, err)
			equal, err := compose.SemanticYAMLEqual(reconstructed, []byte(result.NewEffective))
			require.NoError(t, err)
			require.True(t, equal)

			if algorithm == "intent" {
				require.NotNil(t, result.Diagnostics)
			} else {
				require.Nil(t, result.Diagnostics)
			}
		})
	}
}

func TestFixtureSearchAndLookup(t *testing.T) {
	service := testService(t)
	page := service.ListFixtures("service.image", "atomic", 1, 10)
	require.Greater(t, page.Total, 0)
	require.LessOrEqual(t, len(page.Items), 10)
	for _, item := range page.Items {
		require.Equal(t, "atomic", item.Class)
	}

	fixture, ok := service.Fixture(testFixtureID)
	require.True(t, ok)
	require.Equal(t, testFixtureID, fixture.Metadata.ID)
	require.NotEmpty(t, fixture.Expected)

	_, ok = service.Fixture("../../go.mod")
	require.False(t, ok)
}

func TestMergeRejectsInvalidInput(t *testing.T) {
	service := testService(t)
	_, err := service.Merge(MergeRequest{Algorithm: "unknown", PreviousBase: "{}", TargetBase: "{}"})
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, "request", requestErr.Stage)

	_, err = service.Merge(MergeRequest{Algorithm: "intent", PreviousBase: "[", TargetBase: "{}"})
	require.ErrorAs(t, err, &requestErr)
	require.Equal(t, "previous_effective", requestErr.Stage)
}
