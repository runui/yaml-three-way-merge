package merge_test

import (
	"errors"
	"testing"

	"github.com/runui/yaml-three-way-merge/merge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagesAreReusableAndOwnTheirData(t *testing.T) {
	input := merge.Input{
		PreviousBase: []byte("services:\n  app:\n    image: old\n    ports: [8080:80]\n"),
		UserOverride: []byte("services:\n  app:\n    ports: !reset []\n---\nservices:\n  app:\n    ports: [9090:80]\n"),
		TargetBase:   []byte("services:\n  app:\n    image: new\n    ports: [8080:80, 8081:81]\n"),
	}
	want, err := merge.Merge(input)
	require.NoError(t, err)
	scenario, err := merge.Compile(input)
	require.NoError(t, err)
	// Mutating caller buffers after compilation must not corrupt retained bases.
	for _, buffer := range [][]byte{input.PreviousBase, input.UserOverride, input.TargetBase} {
		for i := range buffer {
			buffer[i] = '!'
		}
	}
	for range 2 {
		direct, err := scenario.ApplyDirect()
		require.NoError(t, err)
		equal, err := merge.Equivalent(direct, want.YAML)
		require.NoError(t, err)
		require.True(t, equal)
		plan, err := scenario.Analyze()
		require.NoError(t, err)
		report, err := plan.Report()
		require.NoError(t, err)
		assert.True(t, report.UserReplayValid)
		require.NotEmpty(t, report.User.Paths)
		report.User.Paths[0] = "corrupted"
		for range 2 {
			got, err := plan.Apply()
			require.NoError(t, err)
			assert.Equal(t, want, got)
			got.YAML[0] = '!'
			got.Report.User.Paths[0] = "corrupted"
		}
	}
}

func TestStageErrorsAndZeroValues(t *testing.T) {
	_, err := merge.Compile(merge.Input{PreviousBase: []byte("[invalid")})
	var stageErr *merge.Error
	require.ErrorAs(t, err, &stageErr)
	assert.Equal(t, merge.StageCompile, stageErr.Stage)
	var scenario *merge.Scenario
	_, err = scenario.Analyze()
	assert.ErrorIs(t, err, merge.ErrUninitialized)
	_, err = (&merge.Scenario{}).ApplyDirect()
	assert.ErrorIs(t, err, merge.ErrUninitialized)
	var plan *merge.Plan
	_, err = plan.Apply()
	assert.True(t, errors.Is(err, merge.ErrUninitialized))
	_, err = (&merge.Plan{}).Report()
	assert.ErrorIs(t, err, merge.ErrUninitialized)
	_, err = merge.Equivalent([]byte("[invalid"), nil)
	require.ErrorAs(t, err, &stageErr)
	assert.Equal(t, merge.StageCompare, stageErr.Stage)
}
