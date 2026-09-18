package corpus

import (
	"fmt"
	"strings"
)

// Alternative syntaxes for unique-resource list fields: the long mapping form
// can be written as a short string (volumes, configs, secrets) and the short
// device string can be written as a long mapping.
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
	registerVariants("service.devices", FieldVariant{
		Name:          "long",
		Render:        renderDevicesLong,
		ResetBoundary: []string{"services", "app", "devices"},
	})
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

// renderDevicesLong expands "/dev/x:/dev/y[:perms]" into the long mapping form.
func renderDevicesLong(field FieldSpec, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make([]any, 0, len(values))
	for _, raw := range values {
		device, ok := raw.(string)
		if !ok {
			encoded = append(encoded, raw)
			continue
		}
		parts := strings.Split(device, ":")
		source := parts[0]
		target := source
		permissions := "rwm"
		if len(parts) > 1 {
			target = parts[1]
		}
		if len(parts) > 2 && parts[2] != "" {
			permissions = parts[2]
		}
		encoded = append(encoded, map[string]any{
			"source":      source,
			"target":      target,
			"permissions": permissions,
		})
	}
	return mappingPath(field.Path, encoded)
}
