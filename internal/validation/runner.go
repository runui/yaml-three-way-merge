package validation

import (
	"fmt"

	internalcompose "github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/internal/compose"
	packoverride "github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/override"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/smdmerge"
)

type Result struct {
	Case                     Case   `json:"case"`
	OldOverride              string `json:"old_override,omitempty"`
	ReconstructedUser        string `json:"reconstructed_user,omitempty"`
	NewOverride              string `json:"new_override,omitempty"`
	Actual                   string `json:"actual,omitempty"`
	ReconstructionOK         bool   `json:"reconstruction_ok"`
	MatchesDesired           bool   `json:"matches_desired"`
	Idempotent               bool   `json:"idempotent"`
	MatchesClassification    bool   `json:"matches_classification"`
	SMDActual                string `json:"smd_actual,omitempty"`
	SMDMatchesDesired        bool   `json:"smd_matches_desired"`
	SMDMatchesClassification bool   `json:"smd_matches_classification"`
	SMDError                 string `json:"smd_error,omitempty"`
	Error                    string `json:"error,omitempty"`
}

func RunAll(cases []Case) []Result {
	results := make([]Result, 0, len(cases))
	for _, testCase := range cases {
		results = append(results, Run(testCase))
	}
	return results
}

func Run(testCase Case) Result {
	result := Result{Case: testCase}
	oldOverride, err := packoverride.Build([]byte(testCase.BaseOld), []byte(testCase.User))
	if err != nil {
		result.Error = fmt.Sprintf("build old override: %v", err)
		return result
	}
	result.OldOverride = string(oldOverride)

	reconstructed, err := internalcompose.MergeYAML([]byte(testCase.BaseOld), oldOverride)
	if err != nil {
		result.Error = fmt.Sprintf("reconstruct user: %v", err)
		return result
	}
	result.ReconstructedUser = string(reconstructed)
	result.ReconstructionOK, err = internalcompose.SemanticYAMLEqual(reconstructed, []byte(testCase.User))
	if err != nil {
		result.Error = fmt.Sprintf("compare reconstructed user: %v", err)
		return result
	}

	newOverride, err := packoverride.RebaseRepositoryUpdate(
		[]byte(testCase.BaseOld),
		[]byte(testCase.BaseNew),
		oldOverride,
		packoverride.RepositoryRebaseOptions{},
	)
	if err != nil {
		result.Error = fmt.Sprintf("rebase repository update: %v", err)
		return result
	}
	result.NewOverride = string(newOverride)

	actual, err := internalcompose.MergeYAML([]byte(testCase.BaseNew), newOverride)
	if err != nil {
		result.Error = fmt.Sprintf("merge rebased override: %v", err)
		return result
	}
	result.Actual = string(actual)
	result.MatchesDesired, err = internalcompose.SemanticYAMLEqual(actual, []byte(testCase.Expected))
	if err != nil {
		result.Error = fmt.Sprintf("compare desired result: %v", err)
		return result
	}

	secondOverride, err := packoverride.RebaseRepositoryUpdate(
		[]byte(testCase.BaseNew),
		[]byte(testCase.BaseNew),
		newOverride,
		packoverride.RepositoryRebaseOptions{},
	)
	if err != nil {
		result.Error = fmt.Sprintf("check idempotence: %v", err)
		return result
	}
	secondActual, err := internalcompose.MergeYAML([]byte(testCase.BaseNew), secondOverride)
	if err != nil {
		result.Error = fmt.Sprintf("merge idempotence result: %v", err)
		return result
	}
	result.Idempotent, err = internalcompose.SemanticYAMLEqual(actual, secondActual)
	if err != nil {
		result.Error = fmt.Sprintf("compare idempotence result: %v", err)
		return result
	}

	result.MatchesClassification = result.ReconstructionOK && result.Idempotent &&
		(testCase.Expectation == ExpectationMatch && result.MatchesDesired ||
			testCase.Expectation == ExpectationKnownGap && !result.MatchesDesired)

	smdActual, smdErr := smdmerge.Merge([]byte(testCase.BaseOld), []byte(testCase.User), []byte(testCase.BaseNew))
	if smdErr != nil {
		result.SMDError = smdErr.Error()
		return result
	}
	result.SMDActual = string(smdActual)
	result.SMDMatchesDesired, smdErr = smdmerge.Equivalent(smdActual, []byte(testCase.Expected))
	if smdErr != nil {
		result.SMDError = smdErr.Error()
		return result
	}
	smdExpectation := testCase.ExpectedSMDClassification()
	result.SMDMatchesClassification = smdExpectation == ExpectationMatch && result.SMDMatchesDesired ||
		smdExpectation == ExpectationKnownGap && !result.SMDMatchesDesired
	return result
}

func HasDesiredMismatch(results []Result) bool {
	for _, result := range results {
		if result.Error != "" || result.SMDError != "" || !result.ReconstructionOK || !result.Idempotent || !result.MatchesDesired || !result.SMDMatchesDesired {
			return true
		}
	}
	return false
}

func HasClassificationMismatch(results []Result) bool {
	for _, result := range results {
		if result.Error != "" || result.SMDError != "" || !result.MatchesClassification || !result.SMDMatchesClassification {
			return true
		}
	}
	return false
}
