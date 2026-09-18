package corpus

import "strings"

// Map-like fields written as a mapping ({key: value}) instead of the default
// list of "key=value" scalars. The logical items are shared, only the authored
// syntax changes.
func init() {
	registerMapVariant("service.environment-list", []string{"services", "app", "environment"})
	registerMapVariant("service.labels-list", []string{"services", "app", "labels"})
	registerMapVariant("service.annotations-list", []string{"services", "app", "annotations"})
	registerMapVariant("service.sysctls-list", []string{"services", "app", "sysctls"})
	registerMapVariant("service.extra-hosts-list", []string{"services", "app", "extra_hosts"})
	registerMapVariant("build.args-list", []string{"services", "app", "build", "args"})
	registerMapVariant("build.labels-list", []string{"services", "app", "build", "labels"})
	registerMapVariant("build.additional-contexts-list", []string{"services", "app", "build", "additional_contexts"})
	registerMapVariant("build.extra-hosts-list", []string{"services", "app", "build", "extra_hosts"})
	registerMapVariant("network.labels-list", []string{"networks", "default", "labels"})
	registerMapVariant("volume.labels-list", []string{"volumes", "default", "labels"})
	registerMapVariant("config.labels-list", []string{"configs", "default", "labels"})
	registerMapVariant("secret.labels-list", []string{"secrets", "default", "labels"})
}

func registerMapVariant(fieldID string, path []string) {
	registerVariants(fieldID, FieldVariant{
		Name: "map",
		Render: func(field FieldSpec, values []any, present bool) map[string]any {
			return renderKeyValueMap(path, values, present)
		},
		ResetBoundary: path,
	})
}

func renderKeyValueMap(path []string, values []any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	encoded := make(map[string]any, len(values))
	for _, raw := range values {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		key, value, _ := strings.Cut(text, "=")
		encoded[key] = value
	}
	return mappingPath(path, encoded)
}
