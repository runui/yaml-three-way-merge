package corpus

import "fmt"

// Ports written in short syntax: "published:target".
func init() {
	registerVariants("service.ports", FieldVariant{
		Name:          "short",
		Render:        renderPortsShort,
		ResetBoundary: []string{"services", "app", "ports"},
	})
}

func renderPortsShort(field FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make([]any, 0, len(values))
	for _, raw := range values {
		port, ok := raw.(map[string]any)
		if !ok {
			encoded = append(encoded, raw)
			continue
		}
		encoded = append(encoded, fmt.Sprintf("%v:%v", port["published"], port["target"]))
	}
	return map[string]any{"services": map[string]any{"app": map[string]any{"ports": encoded}}}
}
