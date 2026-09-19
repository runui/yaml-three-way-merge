package smdmodel

import (
	"fmt"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
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
	output := cloneUnstructured(value.AsValue().Unstructured())
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

// cloneUnstructured copies containers without a YAML round trip. Scalar types
// must survive unchanged, and output cleanup must never mutate a compiled value.
func cloneUnstructured(value any) any {
	switch current := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(current))
		for key, child := range current {
			clone[key] = cloneUnstructured(child)
		}
		return clone
	case []any:
		clone := make([]any, len(current))
		for i, child := range current {
			clone[i] = cloneUnstructured(child)
		}
		return clone
	default:
		return value
	}
}
