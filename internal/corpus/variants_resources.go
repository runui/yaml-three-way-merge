package corpus

import "fmt"

// Alternative syntaxes for unique-resource list fields. service.volumes accepts
// the short "source:target" string as well as the long mapping form. service
// devices has no long form in the pinned Compose version: compose-go v1.20.2
// types service devices as a list of strings only, so a mapping item is not
// valid Compose and no @long variant is registered.
func init() {
	registerVariants("service.volumes", FieldVariant{
		Name:          "short",
		Render:        renderVolumesShort,
		ResetBoundary: []string{"services", "app", "volumes"},
	})
	// configs/secrets do not get a short-string variant: the short form carries
	// a single name that is both source and target, so it cannot represent the
	// stable identity and the modified value as separate fields, which the
	// logical-items semantics requires.
}

// renderVolumesShort encodes bind mounts as "source:target".
func renderVolumesShort(field FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make([]any, 0, len(values))
	for _, raw := range values {
		volume, ok := raw.(map[string]any)
		if !ok {
			encoded = append(encoded, raw)
			continue
		}
		encoded = append(encoded, fmt.Sprintf("%v:%v", volume["source"], volume["target"]))
	}
	return mappingPath(field.Path, encoded)
}
