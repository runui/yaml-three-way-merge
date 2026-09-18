package corpus

import (
	"fmt"
	"strings"
)

// Scalar and string syntax variants for Compose fields that accept both an
// atomic scalar/shell string and a sequence form.
func init() {
	registerVariants("service.command", FieldVariant{
		Name:          "string",
		Render:        renderScalarString,
		ResetBoundary: []string{"services", "app", "command"},
	})
	registerVariants("service.entrypoint", FieldVariant{
		Name:          "string",
		Render:        renderScalarString,
		ResetBoundary: []string{"services", "app", "entrypoint"},
	})
	registerVariants("healthcheck.test", FieldVariant{
		Name:          "string",
		Render:        renderScalarString,
		ResetBoundary: []string{"services", "app", "healthcheck", "test"},
	})
	registerVariants("service.env-file", FieldVariant{
		Name:          "scalar",
		Render:        renderScalarList,
		ResetBoundary: []string{"services", "app", "env_file"},
	})
	registerVariants("service.tmpfs", FieldVariant{
		Name:          "scalar",
		Render:        renderScalarList,
		ResetBoundary: []string{"services", "app", "tmpfs"},
	})
}

// renderScalarString joins every present logical value into a single
// space-separated string. These fields are atomic, so values carries at most
// one element; a nested sequence element (for example a healthcheck command)
// is flattened as well.
func renderScalarString(field FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, scalarText(value))
	}
	return mappingPath(field.Path, strings.Join(parts, " "))
}

// renderScalarList emits a bare scalar when exactly one logical value is
// present and otherwise falls back to the sequence form. Ordered-list fields
// can carry two elements through the pair cross product, where a scalar is no
// longer a faithful encoding.
func renderScalarList(field FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	if len(values) == 1 {
		return mappingPath(field.Path, scalarText(values[0]))
	}
	return mappingPath(field.Path, values)
}

func scalarText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, " ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, scalarText(item))
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprint(value)
	}
}
