package smdmerge

import (
	"fmt"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/structured-merge-diff/v7/typed"
)

// TypedScenario exposes the normalized states used by the SMD implementation.
// All values share the same schema and can be compared or merged directly.
type TypedScenario struct {
	Old            *typed.TypedValue
	UserPrediction *typed.TypedValue
	New            *typed.TypedValue
}

// CompileTypedScenario compiles the real multi-document user override directly
// into a typed value. It does not materialize user YAML through Compose merge.
func CompileTypedScenario(baseOldYAML, userOverrideYAML, baseNewYAML []byte) (TypedScenario, error) {
	baseOld, err := normalizeYAML(baseOldYAML)
	if err != nil {
		return TypedScenario{}, fmt.Errorf("normalize previous base: %w", err)
	}
	baseNew, err := normalizeYAML(baseNewYAML)
	if err != nil {
		return TypedScenario{}, fmt.Errorf("normalize target base: %w", err)
	}
	if err := assignPortMergeIDs(baseOld, map[string]any{}, baseNew); err != nil {
		return TypedScenario{}, err
	}
	if err := assignStringSetMergeIDs(baseOld, map[string]any{}, baseNew); err != nil {
		return TypedScenario{}, err
	}
	parseable, err := composeType()
	if err != nil {
		return TypedScenario{}, err
	}
	oldValue, err := parseable.FromUnstructured(baseOld)
	if err != nil {
		return TypedScenario{}, fmt.Errorf("type previous base: %w", err)
	}
	newValue, err := parseable.FromUnstructured(baseNew)
	if err != nil {
		return TypedScenario{}, fmt.Errorf("type target base: %w", err)
	}
	userValue, err := applyOverride(parseable, oldValue, baseOld, baseNew, userOverrideYAML)
	if err != nil {
		return TypedScenario{}, fmt.Errorf("compile user override: %w", err)
	}
	return TypedScenario{Old: oldValue, UserPrediction: userValue, New: newValue}, nil
}

// MarshalTypedValue removes SMD-only identity fields and emits Compose YAML.
func MarshalTypedValue(value *typed.TypedValue) ([]byte, error) {
	output, err := cloneUnstructured(value.AsValue().Unstructured())
	if err != nil {
		return nil, fmt.Errorf("clone typed result: %w", err)
	}
	pruneNilContainers(output)
	pruneEmptyContainers(output)
	output = unwrapInternalLists(output)
	stripInternalFields(output)
	content, err := yaml.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal typed result: %w", err)
	}
	return content, nil
}

func cloneUnstructured(value any) (any, error) {
	content, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var clone any
	if err := yaml.Unmarshal(content, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}
