package smdmerge

import (
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/structured-merge-diff/v7/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v7/typed"
)

var (
	parseOnce sync.Once
	parseType typed.ParseableType
	parseErr  error
)

func Merge(baseOldYAML, userOverrideYAML, baseNewYAML []byte) ([]byte, error) {
	baseOld, err := normalizeYAML(baseOldYAML)
	if err != nil {
		return nil, fmt.Errorf("normalize previous base: %w", err)
	}
	baseNew, err := normalizeYAML(baseNewYAML)
	if err != nil {
		return nil, fmt.Errorf("normalize target base: %w", err)
	}
	if err := assignPortMergeIDs(baseOld, map[string]any{}, baseNew); err != nil {
		return nil, err
	}
	if err := assignStringSetMergeIDs(baseOld, map[string]any{}, baseNew); err != nil {
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
	newValue, err := parseable.FromUnstructured(baseNew)
	if err != nil {
		return nil, fmt.Errorf("type target base: %w", err)
	}

	userValue, err := applyOverride(parseable, oldValue, baseOld, baseNew, userOverrideYAML)
	if err != nil {
		return nil, fmt.Errorf("compile user override: %w", err)
	}
	delta, err := oldValue.Compare(userValue)
	if err != nil {
		return nil, fmt.Errorf("compare user changes: %w", err)
	}
	result := newValue.RemoveItems(delta.Removed)
	writes := delta.Modified.Union(delta.Added)
	writes, err = expandToValueLeaves(writes, userValue)
	if err != nil {
		return nil, fmt.Errorf("expand user writes: %w", err)
	}
	patch := userValue.ExtractItems(writes, typed.WithAppendKeyFields())
	result, err = result.Merge(patch)
	if err != nil {
		return nil, fmt.Errorf("apply user changes: %w", err)
	}
	output := result.AsValue().Unstructured()
	pruneNilContainers(output)
	output = unwrapInternalLists(output)
	stripInternalFields(output)
	content, err := yaml.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return content, nil
}

func expandToValueLeaves(paths *fieldpath.Set, value *typed.TypedValue) (*fieldpath.Set, error) {
	valueFields, err := value.ToFieldSet()
	if err != nil {
		return nil, err
	}
	valueLeaves := valueFields.Leaves()
	result := fieldpath.NewSet()
	paths.Iterate(func(path fieldpath.Path) {
		matched := false
		valueLeaves.Iterate(func(candidate fieldpath.Path) {
			if pathPrefix(path, candidate) {
				result.Insert(candidate)
				matched = true
			}
		})
		if !matched {
			result.Insert(path)
		}
	})
	return result, nil
}

func pathPrefix(prefix, path fieldpath.Path) bool {
	if len(prefix) > len(path) {
		return false
	}
	for index := range prefix {
		if !prefix[index].Equals(path[index]) {
			return false
		}
	}
	return true
}

func applyOverride(parseable typed.ParseableType, baseValue *typed.TypedValue, baseOld, baseNew map[string]any, content []byte) (*typed.TypedValue, error) {
	documents, err := parseOverrideDocuments(content)
	if err != nil {
		return nil, err
	}
	result := baseValue
	for index, document := range documents {
		if len(document.resets) > 0 {
			paths := make([]fieldpath.Path, 0, len(document.resets))
			for _, reset := range document.resets {
				parts := make([]interface{}, len(reset))
				for i, part := range reset {
					parts[i] = part
				}
				path, err := fieldpath.MakePath(parts...)
				if err != nil {
					return nil, fmt.Errorf("document %d reset %v: %w", index+1, reset, err)
				}
				paths = append(paths, path)
			}
			result = result.RemoveItems(fieldpath.NewSet(paths...))
		}
		if len(document.yaml) == 0 {
			continue
		}
		patch, err := normalizeYAML(document.yaml)
		if err != nil {
			return nil, fmt.Errorf("document %d normalize writes: %w", index+1, err)
		}
		if len(patch) == 0 {
			continue
		}
		if err := assignPortMergeIDs(baseOld, patch, baseNew); err != nil {
			return nil, fmt.Errorf("document %d assign identities: %w", index+1, err)
		}
		if err := assignStringSetMergeIDs(baseOld, patch, baseNew); err != nil {
			return nil, fmt.Errorf("document %d assign string identities: %w", index+1, err)
		}
		patchValue, err := parseable.FromUnstructured(patch)
		if err != nil {
			return nil, fmt.Errorf("document %d type writes: %w", index+1, err)
		}
		result, err = result.Merge(patchValue)
		if err != nil {
			return nil, fmt.Errorf("document %d apply writes: %w", index+1, err)
		}
	}
	return result, nil
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
	pruneEmptyContainers(left)
	pruneEmptyContainers(right)
	// Assigning both sides as unchanged states gives canonical port IDs without
	// making representation-only differences significant.
	if err := assignPortMergeIDs(left, left, right); err != nil {
		return false, err
	}
	if err := assignStringSetMergeIDs(left, left, right); err != nil {
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

func unwrapInternalLists(value any) any {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			current[key] = unwrapInternalLists(child)
		}
		return current
	case []any:
		for index, child := range current {
			if item, ok := child.(map[string]any); ok {
				if scalar, exists := item["__value"].(string); exists && item["__merge_id"] != nil {
					current[index] = scalar
					continue
				}
			}
			current[index] = unwrapInternalLists(child)
		}
		return current
	default:
		return value
	}
}

// pruneEmptyContainers removes empty mappings and sequences so that an emptied
// list is equivalent to an absent one. Without this, replaying a removal that
// empties a list drops the container while the user prediction keeps it.
func pruneEmptyContainers(value any) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			pruneEmptyContainers(child)
			if isEmptyContainer(child) {
				delete(current, key)
			}
		}
	case []any:
		for _, child := range current {
			pruneEmptyContainers(child)
		}
	}
}

func isEmptyContainer(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		return len(current) == 0
	case []any:
		return len(current) == 0
	default:
		return false
	}
}

func pruneNilContainers(value any) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if child == nil {
				delete(current, key)
				continue
			}
			pruneNilContainers(child)
			switch nested := child.(type) {
			case map[string]any:
				if len(nested) == 0 && key != "services" {
					delete(current, key)
				}
			}
		}
	case []any:
		for _, child := range current {
			pruneNilContainers(child)
		}
	}
}
