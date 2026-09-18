package corpus

import "fmt"

// depends-on and networks written in long syntax: a mapping keyed by name
// instead of the short list-of-names form.
func init() {
	registerVariants("service.depends-on", FieldVariant{
		Name:          "map",
		Render:        renderDependsOnMap,
		ResetBoundary: []string{"services", "app", "depends_on"},
	})
	registerVariants("service.networks", FieldVariant{
		Name:          "map",
		Render:        renderNetworksMap,
		ResetBoundary: []string{"services", "app", "networks"},
	})
}

func renderDependsOnMap(_ FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make(map[string]any, len(values))
	for _, raw := range values {
		encoded[namedMapKey(raw)] = map[string]any{"condition": "service_started", "required": true}
	}
	return map[string]any{"services": map[string]any{"app": map[string]any{"depends_on": encoded}}}
}

func renderNetworksMap(_ FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make(map[string]any, len(values))
	for _, raw := range values {
		encoded[namedMapKey(raw)] = map[string]any{}
	}
	return map[string]any{"services": map[string]any{"app": map[string]any{"networks": encoded}}}
}

func namedMapKey(raw any) string {
	if name, ok := raw.(string); ok {
		return name
	}
	return fmt.Sprintf("%v", raw)
}
