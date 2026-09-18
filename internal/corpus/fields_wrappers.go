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
		if !present {
			return map[string]any{}
		}
		item := map[string]any{"source": "/host", "target": "/container"}
		// type is one of the modeled values, so only inject it as context for
		// the other volume attributes.
		if key != "type" {
			item["type"] = "bind"
		}
		setNestedValue(item, strings.Split(key, "."), values[0])
		return serviceMapping("volumes", []any{item})
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
		if !present {
			return map[string]any{}
		}
		dependency := map[string]any{key: values[0]}
		return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "depends_on": map[string]any{"default": dependency}}}}
	}
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
