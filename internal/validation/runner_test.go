package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMergeBaselineMatrix is the executable logic baseline shared by the
// current implementation and the structured-merge-diff experiment.
func TestMergeBaselineMatrix(t *testing.T) {
	results := RunAll(Cases())
	for _, result := range results {
		t.Run(result.Case.ID, func(t *testing.T) {
			assert.True(t, result.ReconstructionOK, "Build + MergeYAML must reconstruct user.yaml\nreconstructed:\n%s\nuser:\n%s", result.ReconstructedUser, result.Case.User)
			assert.True(t, result.Idempotent, "rebase must be idempotent\nactual:\n%s\nnew override:\n%s", result.Actual, result.NewOverride)

			t.Run("current", func(t *testing.T) {
				assert.NoError(t, errorFromResult(result))
				assert.True(t, result.MatchesClassification,
					"classification changed: expected %s, desired match=%t\nactual:\n%s\ndesired:\n%s\nnew override:\n%s",
					result.Case.Expectation, result.MatchesDesired, result.Actual, result.Case.Expected, result.NewOverride)
			})

			t.Run("structured-merge-diff", func(t *testing.T) {
				assert.Empty(t, result.SMDError)
				assert.True(t, result.SMDMatchesClassification,
					"classification changed: expected %s, desired match=%t\nactual:\n%s\ndesired:\n%s",
					result.Case.ExpectedSMDClassification(), result.SMDMatchesDesired, result.SMDActual, result.Case.Expected)
			})
		})
	}
}

func errorFromResult(result Result) error {
	if result.Error == "" {
		return nil
	}
	return resultError(result.Error)
}

type resultError string

func (err resultError) Error() string { return string(err) }
