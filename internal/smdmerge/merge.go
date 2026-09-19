// Package smdmerge applies the user's typed delta directly to the new base.
package smdmerge

import (
	"fmt"

	"github.com/runui/yaml-three-way-merge/internal/smdmodel"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

func Merge(baseOldYAML, userOverrideYAML, baseNewYAML []byte) ([]byte, error) {
	scenario, err := smdmodel.CompileTypedScenario(baseOldYAML, userOverrideYAML, baseNewYAML)
	if err != nil {
		return nil, err
	}
	return Apply(scenario)
}

// Apply executes the direct strategy on an already compiled scenario.
func Apply(scenario smdmodel.TypedScenario) ([]byte, error) {
	delta, err := scenario.Old.Compare(scenario.UserPrediction)
	if err != nil {
		return nil, fmt.Errorf("compare user changes: %w", err)
	}
	result := scenario.New.RemoveItems(delta.Removed)
	writes, err := smdmodel.ExpandToValueLeaves(delta.Modified.Union(delta.Added), scenario.UserPrediction)
	if err != nil {
		return nil, fmt.Errorf("expand user writes: %w", err)
	}
	patch := scenario.UserPrediction.ExtractItems(writes, typed.WithAppendKeyFields())
	result, err = result.Merge(patch)
	if err != nil {
		return nil, fmt.Errorf("apply user changes: %w", err)
	}
	return smdmodel.MarshalDirectResult(result)
}

func Equivalent(leftYAML, rightYAML []byte) (bool, error) {
	return smdmodel.Equivalent(leftYAML, rightYAML)
}
