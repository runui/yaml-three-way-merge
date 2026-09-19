package corpus

import "strings"

// This file rebuilds the surrounding array container for scalar and mapping
// leaves that live inside array items. The item identity used by the sequence
// corpus (port target, volume target, file reference target, device driver,
// generic resource, ipam subnet, watch path, dependency name) is injected as
// fixed context so only the modeled attribute varies.
//
// Presence of the modeled attribute is expressed by presence of the enclosing
// item: when the logical value is absent the item is omitted entirely. This
// keeps the 15-state matrix unambiguous for scalar attributes, which cannot be
// represented as an empty container.

func wrapIncludeField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		item := map[string]any{"path": []string{"compose.yml"}}
		if present {
			item[key] = values[0]
		}
		return map[string]any{"include": []any{item}}
	}
}

func wrapPortField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		item := map[string]any{"target": 80, "published": "8080"}
		// protocol is one of the modeled values, so only inject it as context
		// for the other port attributes.
		if key != "protocol" {
			item["protocol"] = "tcp"
		}
		item[key] = values[0]
		return serviceMapping("ports", []any{item})
	}
}

func wrapVolumeField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		// A named volume used by type: volume must be declared, so a fixed
		// volume definition is carried as constant context even when the item
		// itself is absent. Source "data" is a valid relative bind path and a
		// valid named-volume reference, so the same item is valid for every
		// value type takes.
		context := map[string]any{}
		if key == "type" {
			context["volumes"] = map[string]any{"data": map[string]any{}}
		}
		if !present {
			return context
		}
		item := map[string]any{"source": "/host", "target": "/container"}
		// type is one of the modeled values, so only inject it as context for
		// the other volume attributes.
		if key != "type" {
			item["type"] = "bind"
		} else {
			item["source"] = "data"
		}
		setNestedValue(item, strings.Split(key, "."), values[0])
		root := serviceMapping("volumes", []any{item})
		for name, value := range context {
			root[name] = value
		}
		return root
	}
}

func wrapFileReferenceField(field, key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		item := map[string]any{"source": "base", "target": "/run/config", key: values[0]}
		return serviceMapping(field, []any{item})
	}
}

func wrapBuildSecretField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		item := map[string]any{"source": "base", "target": "/run/secrets/base", key: values[0]}
		return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "build": map[string]any{"secrets": []any{item}}}}}
	}
}

func wrapDeviceRequestField(branch, key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		device := map[string]any{"driver": "nvidia", "capabilities": []any{"gpu"}, key: values[0]}
		return serviceMapping("deploy", map[string]any{"resources": map[string]any{branch: map[string]any{"devices": []any{device}}}})
	}
}

func wrapGenericResourceKind(branch string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		spec := map[string]any{"kind": values[0], "value": 1}
		return serviceMapping("deploy", map[string]any{"resources": map[string]any{branch: map[string]any{"generic_resources": []any{map[string]any{"discrete_resource_spec": spec}}}}})
	}
}

func wrapIpamConfigField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		config := map[string]any{"subnet": "10.0.0.0/24", key: values[0]}
		return map[string]any{"networks": map[string]any{"default": map[string]any{"ipam": map[string]any{"config": []any{config}}}}}
	}
}

func wrapDevelopWatchField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		item := map[string]any{"path": "./src", key: values[0]}
		return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "develop": map[string]any{"watch": []any{item}}}}}
	}
}

func wrapDependsOnField(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if key == "condition" && !present {
			// condition is the required member of a long-form dependency; when
			// it disappears the whole dependency does.
			return map[string]any{}
		}
		dependency := map[string]any{"condition": "service_started"}
		if key == "condition" {
			dependency["condition"] = values[0]
		} else if present {
			dependency[key] = values[0]
		}
		return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "depends_on": map[string]any{"default": dependency}}}}
	}
}

// wrapExtendsFile models the `file` member of the extends mapping. The mapping
// requires a `service`, so the referenced service is fixed context and only the
// file path varies.
func wrapExtendsFile(values []any, present bool) map[string]any {
	extends := map[string]any{"service": "base"}
	if present {
		extends["file"] = values[0]
	}
	return serviceMapping("extends", extends)
}

func buildIpamAuxAddresses(merged map[string]any, present bool) map[string]any {
	if !present {
		return map[string]any{}
	}
	config := map[string]any{"subnet": "10.0.0.0/24", "aux_addresses": merged}
	return map[string]any{"networks": map[string]any{"default": map[string]any{"ipam": map[string]any{"config": []any{config}}}}}
}

func buildDeviceOptions(branch string) func(map[string]any, bool) map[string]any {
	return func(merged map[string]any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		device := map[string]any{"driver": "nvidia", "capabilities": []any{"gpu"}, "options": merged}
		return serviceMapping("deploy", map[string]any{"resources": map[string]any{branch: map[string]any{"devices": []any{device}}}})
	}
}
