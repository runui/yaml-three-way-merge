package corpus

import (
	"sort"
	"strings"
)

// variantRegistry holds the alternative syntax encodings registered for a field
// ID. Variant files call registerVariants from init, so groups can be added
// independently without editing this file.
var variantRegistry = map[string][]FieldVariant{}

func registerVariants(fieldID string, variants ...FieldVariant) {
	variantRegistry[fieldID] = append(variantRegistry[fieldID], variants...)
}

// VariantFieldID is the stable, filesystem-safe identifier for a variant case.
func VariantFieldID(fieldID, variant string) string {
	if variant == "" {
		return fieldID
	}
	return fieldID + "@" + variant
}

// ArrayFields lists every Compose v1.20.2 and x-casaos field in the corpus:
// sequence fields, sequence short forms, scalar leaves, fixed mappings
// decomposed into leaves, and free-form key maps. Every entry is expanded with
// its registered syntax variants. The name is kept for compatibility with the
// sequence-focused phase; the corpus now covers all Compose fields.
func ArrayFields() []FieldSpec {
	return expandVariants(append(baseArrayFields(), nonArrayFields()...))
}

func expandVariants(base []FieldSpec) []FieldSpec {
	var result []FieldSpec
	for _, field := range base {
		result = append(result, field)
		variants := variantRegistry[field.ID]
		sort.SliceStable(variants, func(i, j int) bool { return variants[i].Name < variants[j].Name })
		for _, variant := range variants {
			expanded := field
			expanded.ID = VariantFieldID(field.ID, variant.Name)
			expanded.Variant = variant.Name
			expanded.Render = variant.Render
			if variant.ResetBoundary != nil {
				expanded.ResetBoundary = append([]string(nil), variant.ResetBoundary...)
			}
			result = append(result, expanded)
		}
	}
	return result
}

// arrayField builds a field with three distinct logical items. Each item has a
// base, user and remote value so the 15-state matrix and the 15x15 pair matrix
// can be generated for the first two items and multi-item scenarios for all
// three.
func arrayField(id, path string, class MergeClass, values [3][3]any) FieldSpec {
	semantics := PairByLogicalItem
	if class == ClassAtomic {
		semantics = PairDisabled
	} else if class == ClassOrderedList {
		semantics = PairAsWholeList
	}
	return FieldSpec{
		ID: id, Path: strings.Split(path, "."), Class: class,
		Values: values[0], SecondValues: values[1], ThirdValues: values[2],
		CrossProduct:  class != ClassAtomic,
		PairSemantics: semantics, ResetBoundary: strings.Split(path, "."),
	}
}

func wrappedArrayField(id, displayPath string, class MergeClass, values [3][3]any, wrap func([]any, bool) map[string]any) FieldSpec {
	field := arrayField(id, displayPath, class, values)
	field.Wrap = wrap
	switch id {
	case "include.path", "include.env-file":
		field.ResetBoundary = []string{"include"}
	case "develop.watch.ignore":
		field.ResetBoundary = []string{"services", "app", "develop", "watch"}
	case "deploy.resources.limits.device.capabilities", "deploy.resources.limits.device-ids":
		field.ResetBoundary = []string{"services", "app", "deploy", "resources", "limits", "devices"}
	case "deploy.resources.reservations.device.capabilities", "deploy.resources.reservations.device-ids":
		field.ResetBoundary = []string{"services", "app", "deploy", "resources", "reservations", "devices"}
	}
	return field
}

// strings3 encodes three logical items, each with base/user/remote values.
func strings3(a, b, c [3]string) [3][3]any {
	return [3][3]any{{a[0], a[1], a[2]}, {b[0], b[1], b[2]}, {c[0], c[1], c[2]}}
}

// baseArrayFields lists every Compose v1.20.2 and x-casaos field whose authored
// form is a sequence or supports a sequence short form.
func baseArrayFields() []FieldSpec {
	return []FieldSpec{
		arrayField("top.include", "include", ClassOrderedList, strings3(
			[3]string{"./base.yml", "./user.yml", "./remote.yml"},
			[3]string{"./base-b.yml", "./user-b.yml", "./remote-b.yml"},
			[3]string{"./base-c.yml", "./user-c.yml", "./remote-c.yml"})),
		wrappedArrayField("include.path", "include[].path", ClassOrderedList, stringValues("base.yml", "user.yml", "remote.yml"), wrapInclude("path")),
		wrappedArrayField("include.env-file", "include[].env_file", ClassOrderedList, stringValues("base.env", "user.env", "remote.env"), wrapInclude("env_file")),

		arrayField("service.annotations-list", "services.app.annotations", ClassNamedItem, keyValueValues("annotation")),
		arrayField("service.profiles", "services.app.profiles", ClassSetList, stringValues("base", "user", "remote")),
		arrayField("service.cap-add", "services.app.cap_add", ClassSetList, stringValues("NET_ADMIN", "SYS_ADMIN", "CHOWN")),
		arrayField("service.cap-drop", "services.app.cap_drop", ClassSetList, stringValues("NET_RAW", "SETUID", "SETGID")),
		arrayField("service.command", "services.app.command", ClassAtomic, commandValues()),
		arrayField("service.entrypoint", "services.app.entrypoint", ClassAtomic, commandValues()),
		arrayField("service.environment-list", "services.app.environment", ClassNamedItem, keyValueValues("ENV")),
		arrayField("service.configs", "services.app.configs", ClassUniqueList, resourceValues("base_config", "user_config", "remote_config", "/config-a", "/config-b", "/config-c")),
		arrayField("service.secrets", "services.app.secrets", ClassUniqueList, resourceValues("base_secret", "user_secret", "remote_secret", "/secret-a", "/secret-b", "/secret-c")),
		arrayField("service.depends-on", "services.app.depends_on", ClassNamedItem, stringValues("base-dependency", "user-dependency", "remote-dependency")),
		arrayField("service.device-cgroup-rules", "services.app.device_cgroup_rules", ClassSetList, stringValues("c 1:3 rwm", "c 1:5 rwm", "c 1:7 rwm")),
		arrayField("service.devices", "services.app.devices", ClassUniqueList, deviceValues()),
		arrayField("service.dns", "services.app.dns", ClassSetList, stringValues("1.1.1.1", "8.8.8.8", "9.9.9.9")),
		arrayField("service.dns-opt", "services.app.dns_opt", ClassSetList, stringValues("use-vc", "rotate", "single-request")),
		arrayField("service.dns-search", "services.app.dns_search", ClassSetList, stringValues("base.test", "user.test", "remote.test")),
		arrayField("service.env-file", "services.app.env_file", ClassOrderedList, stringValues("base.env", "user.env", "remote.env")),
		arrayField("service.expose", "services.app.expose", ClassSetList, stringValues("80", "81", "82")),
		arrayField("service.external-links", "services.app.external_links", ClassSetList, stringValues("base:base", "user:user", "remote:remote")),
		arrayField("service.extra-hosts-list", "services.app.extra_hosts", ClassNamedItem, keyValueValues("host")),
		arrayField("service.group-add", "services.app.group_add", ClassSetList, stringValues("1000", "1001", "1002")),
		arrayField("service.links", "services.app.links", ClassSetList, stringValues("base:base", "user:user", "remote:remote")),
		arrayField("service.labels-list", "services.app.labels", ClassNamedItem, keyValueValues("label")),
		arrayField("service.networks", "services.app.networks", ClassNamedItem, stringValues("base-net", "user-net", "remote-net")),
		arrayField("service.ports", "services.app.ports", ClassUniqueList, portValues()),
		arrayField("service.security-opt", "services.app.security_opt", ClassSetList, stringValues("label=base", "label=user", "label=remote")),
		arrayField("service.sysctls-list", "services.app.sysctls", ClassNamedItem, keyValueValues("net.core.value")),
		arrayField("service.tmpfs", "services.app.tmpfs", ClassUniqueList, stringValues("/base", "/user", "/remote")),
		arrayField("service.volumes", "services.app.volumes", ClassUniqueList, volumeValues()),
		arrayField("service.volumes-from", "services.app.volumes_from", ClassSetList, stringValues("base:ro", "user:ro", "remote:ro")),

		arrayField("build.ssh", "services.app.build.ssh", ClassSetList, stringValues("base", "user", "remote")),
		arrayField("build.args-list", "services.app.build.args", ClassNamedItem, keyValueValues("ARG")),
		arrayField("build.labels-list", "services.app.build.labels", ClassNamedItem, keyValueValues("build.label")),
		arrayField("build.additional-contexts-list", "services.app.build.additional_contexts", ClassNamedItem, keyValueValues("context")),
		arrayField("build.extra-hosts-list", "services.app.build.extra_hosts", ClassNamedItem, keyValueValues("build-host")),
		arrayField("build.cache-from", "services.app.build.cache_from", ClassOrderedList, stringValues("type=local,src=base", "type=local,src=user", "type=local,src=remote")),
		arrayField("build.cache-to", "services.app.build.cache_to", ClassOrderedList, stringValues("type=local,dest=base", "type=local,dest=user", "type=local,dest=remote")),
		arrayField("build.secrets", "services.app.build.secrets", ClassUniqueList, resourceValues("base_secret", "user_secret", "remote_secret", "/run/secrets/a", "/run/secrets/b", "/run/secrets/c")),
		arrayField("build.tags", "services.app.build.tags", ClassOrderedList, stringValues("example:base", "example:user", "example:remote")),
		arrayField("build.platforms", "services.app.build.platforms", ClassOrderedList, strings3(
			[3]string{"linux/amd64", "linux/arm64", "linux/386"},
			[3]string{"linux/arm/v7", "linux/ppc64le", "linux/s390x"},
			[3]string{"linux/riscv64", "linux/mips64le", "linux/arm/v6"})),

		arrayField("develop.watch", "services.app.develop.watch", ClassOrderedList, watchValues()),
		wrappedArrayField("develop.watch.ignore", "services.app.develop.watch[].ignore", ClassOrderedList, stringValues("base.tmp", "user.tmp", "remote.tmp"), wrapDevelopIgnore),
		arrayField("blkio.weight-device", "services.app.blkio_config.weight_device", ClassUniqueList, weightDeviceValues()),
		arrayField("blkio.device-read-bps", "services.app.blkio_config.device_read_bps", ClassUniqueList, throttleValues("1mb")),
		arrayField("blkio.device-read-iops", "services.app.blkio_config.device_read_iops", ClassUniqueList, throttleValues("100")),
		arrayField("blkio.device-write-bps", "services.app.blkio_config.device_write_bps", ClassUniqueList, throttleValues("1mb")),
		arrayField("blkio.device-write-iops", "services.app.blkio_config.device_write_iops", ClassUniqueList, throttleValues("100")),
		arrayField("healthcheck.test", "services.app.healthcheck.test", ClassAtomic, commandValues()),

		arrayField("deploy.resources.limits.devices", "services.app.deploy.resources.limits.devices", ClassUniqueList, deviceRequestValues()),
		wrappedArrayField("deploy.resources.limits.device.capabilities", "services.app.deploy.resources.limits.devices[].capabilities", ClassSetList, stringValues("gpu", "tpu", "npu"), wrapDeviceRequest("limits", "capabilities")),
		wrappedArrayField("deploy.resources.limits.device-ids", "services.app.deploy.resources.limits.devices[].device_ids", ClassSetList, stringValues("GPU-base", "GPU-user", "GPU-remote"), wrapDeviceRequest("limits", "device_ids")),
		arrayField("deploy.resources.limits.generic-resources", "services.app.deploy.resources.limits.generic_resources", ClassUniqueList, genericResourceValues()),
		arrayField("deploy.resources.reservations.devices", "services.app.deploy.resources.reservations.devices", ClassUniqueList, deviceRequestValues()),
		wrappedArrayField("deploy.resources.reservations.device.capabilities", "services.app.deploy.resources.reservations.devices[].capabilities", ClassSetList, stringValues("gpu", "tpu", "npu"), wrapDeviceRequest("reservations", "capabilities")),
		wrappedArrayField("deploy.resources.reservations.device-ids", "services.app.deploy.resources.reservations.devices[].device_ids", ClassSetList, stringValues("GPU-base", "GPU-user", "GPU-remote"), wrapDeviceRequest("reservations", "device_ids")),
		arrayField("deploy.resources.reservations.generic-resources", "services.app.deploy.resources.reservations.generic_resources", ClassUniqueList, genericResourceValues()),
		arrayField("deploy.placement.constraints", "services.app.deploy.placement.constraints", ClassOrderedList, stringValues("node.role==manager", "node.labels.zone==user", "node.labels.zone==remote")),
		arrayField("deploy.placement.preferences", "services.app.deploy.placement.preferences", ClassOrderedList, preferenceValues()),
		arrayField("service.network.aliases", "services.app.networks.default.aliases", ClassSetList, stringValues("base-alias", "user-alias", "remote-alias")),
		arrayField("service.network.link-local-ips", "services.app.networks.default.link_local_ips", ClassSetList, stringValues("169.254.1.1", "169.254.1.2", "169.254.1.3")),

		arrayField("network.ipam.config", "networks.default.ipam.config", ClassUniqueList, ipamValues()),
		arrayField("network.labels-list", "networks.default.labels", ClassNamedItem, keyValueValues("network.label")),
		arrayField("volume.labels-list", "volumes.default.labels", ClassNamedItem, keyValueValues("volume.label")),
		arrayField("config.labels-list", "configs.default.labels", ClassNamedItem, keyValueValues("config.label")),
		arrayField("secret.labels-list", "secrets.default.labels", ClassNamedItem, keyValueValues("secret.label")),
		arrayField("x-casaos.screenshot-link", "x-casaos.screenshot_link", ClassOrderedList, stringValues("base.png", "user.png", "remote.png")),
		arrayField("x-casaos.architectures", "x-casaos.architectures", ClassSetList, stringValues("amd64", "arm64", "386")),
		arrayField("extension.unknown", "x-validation.items", ClassOrderedList, stringValues("base", "user", "remote")),
	}
}

func keyValueValues(key string) [3][3]any {
	return strings3(
		[3]string{key + "-a=base", key + "-a=user", key + "-a=remote"},
		[3]string{key + "-b=base", key + "-b=user", key + "-b=remote"},
		[3]string{key + "-c=base", key + "-c=user", key + "-c=remote"},
	)
}

func wrapInclude(key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		if key == "path" && !present {
			return map[string]any{}
		}
		item := map[string]any{"path": []string{"compose.yml"}}
		if key == "path" || present {
			item[key] = values
		}
		return map[string]any{"include": []any{item}}
	}
}

func wrapDevelopIgnore(values []any, present bool) map[string]any {
	item := map[string]any{"path": ".", "action": "sync"}
	if present {
		item["ignore"] = values
	}
	return map[string]any{"services": map[string]any{"app": map[string]any{"develop": map[string]any{"watch": []any{item}}}}}
}

func wrapDeviceRequest(branch, key string) func([]any, bool) map[string]any {
	return func(values []any, present bool) map[string]any {
		device := map[string]any{"driver": "nvidia", "capabilities": []string{"gpu"}}
		if key == "capabilities" || present {
			device[key] = values
		}
		return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "deploy": map[string]any{"resources": map[string]any{branch: map[string]any{"devices": []any{device}}}}}}}
	}
}

func stringValues(a, u, r string) [3][3]any {
	return strings3(
		[3]string{a + "-a", u + "-a", r + "-a"},
		[3]string{a + "-b", u + "-b", r + "-b"},
		[3]string{a + "-c", u + "-c", r + "-c"},
	)
}

func portValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"target": 80, "published": "8081", "protocol": "tcp"}, map[string]any{"target": 80, "published": "8881", "protocol": "tcp"}, map[string]any{"target": 80, "published": "9081", "protocol": "tcp"}},
		{map[string]any{"target": 81, "published": "8082", "protocol": "tcp"}, map[string]any{"target": 81, "published": "8882", "protocol": "tcp"}, map[string]any{"target": 81, "published": "9082", "protocol": "tcp"}},
		{map[string]any{"target": 82, "published": "8083", "protocol": "tcp"}, map[string]any{"target": 82, "published": "8883", "protocol": "tcp"}, map[string]any{"target": 82, "published": "9083", "protocol": "tcp"}},
	}
}

func volumeValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"type": "bind", "source": "/base-a", "target": "/data-a"}, map[string]any{"type": "bind", "source": "/user-a", "target": "/data-a"}, map[string]any{"type": "bind", "source": "/remote-a", "target": "/data-a"}},
		{map[string]any{"type": "bind", "source": "/base-b", "target": "/data-b"}, map[string]any{"type": "bind", "source": "/user-b", "target": "/data-b"}, map[string]any{"type": "bind", "source": "/remote-b", "target": "/data-b"}},
		{map[string]any{"type": "bind", "source": "/base-c", "target": "/data-c"}, map[string]any{"type": "bind", "source": "/user-c", "target": "/data-c"}, map[string]any{"type": "bind", "source": "/remote-c", "target": "/data-c"}},
	}
}

func resourceValues(base, user, remote, targetA, targetB, targetC string) [3][3]any {
	return [3][3]any{
		{map[string]any{"source": base + "_a", "target": targetA}, map[string]any{"source": user + "_a", "target": targetA}, map[string]any{"source": remote + "_a", "target": targetA}},
		{map[string]any{"source": base + "_b", "target": targetB}, map[string]any{"source": user + "_b", "target": targetB}, map[string]any{"source": remote + "_b", "target": targetB}},
		{map[string]any{"source": base + "_c", "target": targetC}, map[string]any{"source": user + "_c", "target": targetC}, map[string]any{"source": remote + "_c", "target": targetC}},
	}
}

func deviceValues() [3][3]any {
	return strings3(
		[3]string{"/dev/base-a:/dev/a", "/dev/user-a:/dev/a", "/dev/remote-a:/dev/a"},
		[3]string{"/dev/base-b:/dev/b", "/dev/user-b:/dev/b", "/dev/remote-b:/dev/b"},
		[3]string{"/dev/base-c:/dev/c", "/dev/user-c:/dev/c", "/dev/remote-c:/dev/c"})
}

func commandValues() [3][3]any {
	return [3][3]any{
		{[]string{"CMD", "base-a"}, []string{"CMD", "user-a"}, []string{"CMD", "remote-a"}},
		{[]string{"CMD", "base-b"}, []string{"CMD", "user-b"}, []string{"CMD", "remote-b"}},
		{[]string{"CMD", "base-c"}, []string{"CMD", "user-c"}, []string{"CMD", "remote-c"}},
	}
}

func watchValues() [3][3]any {
	return objectTriples("path", "./base-a", "./user-a", "./remote-a", "./base-b", "./user-b", "./remote-b", "./base-c", "./user-c", "./remote-c", "action", "sync")
}

func weightDeviceValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"path": "/dev/a", "weight": 100}, map[string]any{"path": "/dev/a", "weight": 200}, map[string]any{"path": "/dev/a", "weight": 300}},
		{map[string]any{"path": "/dev/b", "weight": 400}, map[string]any{"path": "/dev/b", "weight": 500}, map[string]any{"path": "/dev/b", "weight": 600}},
		{map[string]any{"path": "/dev/c", "weight": 700}, map[string]any{"path": "/dev/c", "weight": 800}, map[string]any{"path": "/dev/c", "weight": 900}},
	}
}

func throttleValues(rate string) [3][3]any {
	return [3][3]any{
		{map[string]any{"path": "/dev/a", "rate": rate}, map[string]any{"path": "/dev/a", "rate": "2" + rate}, map[string]any{"path": "/dev/a", "rate": "3" + rate}},
		{map[string]any{"path": "/dev/b", "rate": "11" + rate}, map[string]any{"path": "/dev/b", "rate": "12" + rate}, map[string]any{"path": "/dev/b", "rate": "13" + rate}},
		{map[string]any{"path": "/dev/c", "rate": "21" + rate}, map[string]any{"path": "/dev/c", "rate": "22" + rate}, map[string]any{"path": "/dev/c", "rate": "23" + rate}},
	}
}

func preferenceValues() [3][3]any {
	return objectTriples("spread", "node.labels.base-a", "node.labels.user-a", "node.labels.remote-a",
		"node.labels.base-b", "node.labels.user-b", "node.labels.remote-b",
		"node.labels.base-c", "node.labels.user-c", "node.labels.remote-c")
}

func deviceRequestValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"driver": "base-a", "capabilities": []string{"gpu"}}, map[string]any{"driver": "user-a", "capabilities": []string{"gpu"}}, map[string]any{"driver": "remote-a", "capabilities": []string{"gpu"}}},
		{map[string]any{"driver": "base-b", "capabilities": []string{"tpu"}}, map[string]any{"driver": "user-b", "capabilities": []string{"tpu"}}, map[string]any{"driver": "remote-b", "capabilities": []string{"tpu"}}},
		{map[string]any{"driver": "base-c", "capabilities": []string{"npu"}}, map[string]any{"driver": "user-c", "capabilities": []string{"npu"}}, map[string]any{"driver": "remote-c", "capabilities": []string{"npu"}}},
	}
}

func genericResourceValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-A", "value": 1}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-A", "value": 2}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-A", "value": 3}}},
		{map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-B", "value": 1}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-B", "value": 2}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-B", "value": 3}}},
		{map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-C", "value": 1}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-C", "value": 2}}, map[string]any{"discrete_resource_spec": map[string]any{"kind": "GPU-C", "value": 3}}},
	}
}

func ipamValues() [3][3]any {
	return [3][3]any{
		{map[string]any{"subnet": "10.1.0.0/24"}, map[string]any{"subnet": "10.2.0.0/24"}, map[string]any{"subnet": "10.3.0.0/24"}},
		{map[string]any{"subnet": "10.11.0.0/24"}, map[string]any{"subnet": "10.12.0.0/24"}, map[string]any{"subnet": "10.13.0.0/24"}},
		{map[string]any{"subnet": "10.21.0.0/24"}, map[string]any{"subnet": "10.22.0.0/24"}, map[string]any{"subnet": "10.23.0.0/24"}},
	}
}

func objectTriples(identity string, ba, ua, ra, bb, ub, rb, bc, uc, rc string, extra ...any) [3][3]any {
	item := func(identityValue string) map[string]any {
		result := map[string]any{identity: identityValue}
		for index := 0; index+1 < len(extra); index += 2 {
			result[extra[index].(string)] = extra[index+1]
		}
		return result
	}
	return [3][3]any{
		{item(ba), item(ua), item(ra)},
		{item(bb), item(ub), item(rb)},
		{item(bc), item(uc), item(rc)},
	}
}
