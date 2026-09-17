package smdmerge

import (
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/structured-merge-diff/v7/typed"
)

var (
	parseOnce sync.Once
	parseType typed.ParseableType
	parseErr  error
)

func Merge(baseOldYAML, userYAML, baseNewYAML []byte) ([]byte, error) {
	baseOld, err := normalizeYAML(baseOldYAML)
	if err != nil {
		return nil, fmt.Errorf("normalize previous base: %w", err)
	}
	user, err := normalizeYAML(userYAML)
	if err != nil {
		return nil, fmt.Errorf("normalize user: %w", err)
	}
	baseNew, err := normalizeYAML(baseNewYAML)
	if err != nil {
		return nil, fmt.Errorf("normalize target base: %w", err)
	}
	if err := assignPortMergeIDs(baseOld, user, baseNew); err != nil {
		return nil, err
	}

	parseable, err := composeType()
	if err != nil {
		return nil, err
	}
	oldValue, err := parseable.FromUnstructured(baseOld)
	if err != nil {
		return nil, fmt.Errorf("type previous base: %w", err)
	}
	userValue, err := parseable.FromUnstructured(user)
	if err != nil {
		return nil, fmt.Errorf("type user: %w", err)
	}
	newValue, err := parseable.FromUnstructured(baseNew)
	if err != nil {
		return nil, fmt.Errorf("type target base: %w", err)
	}

	delta, err := oldValue.Compare(userValue)
	if err != nil {
		return nil, fmt.Errorf("compare user changes: %w", err)
	}
	result := newValue.RemoveItems(delta.Removed)
	writes := delta.Modified.Union(delta.Added)
	patch := userValue.ExtractItems(writes, typed.WithAppendKeyFields())
	result, err = result.Merge(patch)
	if err != nil {
		return nil, fmt.Errorf("apply user changes: %w", err)
	}
	output := result.AsValue().Unstructured()
	stripInternalFields(output)
	content, err := yaml.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return content, nil
}

func Equivalent(leftYAML, rightYAML []byte) (bool, error) {
	left, err := normalizeYAML(leftYAML)
	if err != nil {
		return false, err
	}
	right, err := normalizeYAML(rightYAML)
	if err != nil {
		return false, err
	}
	// Assigning both sides as unchanged states gives canonical port IDs without
	// making representation-only differences significant.
	if err := assignPortMergeIDs(left, left, right); err != nil {
		return false, err
	}
	parseable, err := composeType()
	if err != nil {
		return false, err
	}
	leftValue, err := parseable.FromUnstructured(left)
	if err != nil {
		return false, err
	}
	rightValue, err := parseable.FromUnstructured(right)
	if err != nil {
		return false, err
	}
	comparison, err := leftValue.Compare(rightValue)
	return err == nil && comparison.IsSame(), err
}

func composeType() (typed.ParseableType, error) {
	parseOnce.Do(func() {
		parser, err := typed.NewParser(schemaYAML)
		if err != nil {
			parseErr = fmt.Errorf("parse SMD schema: %w", err)
			return
		}
		parseType = parser.Type("Compose")
	})
	return parseType, parseErr
}

func stripInternalFields(value any) {
	switch current := value.(type) {
	case map[string]any:
		delete(current, "__merge_id")
		delete(current, "__present")
		for _, child := range current {
			stripInternalFields(child)
		}
	case []any:
		for _, child := range current {
			stripInternalFields(child)
		}
	}
}
