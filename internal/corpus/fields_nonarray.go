package corpus

import "strings"

// This file models every Compose v1.20.2 and x-casaos field that is not a
// sequence or sequence short form. Three shapes are covered:
//
//   - scalar leaves modeled as atomic whole values (booleans, integers,
//     strings, durations and enums);
//   - fixed-structure mappings decomposed into their individual leaf paths;
//   - free-form key/value mappings modeled as named logical items.
//
// Expected results are still produced only from the user-intent state matrix in
// states.go. No merge algorithm is consulted while building the corpus, so the
// fixtures can be used to characterize the boundaries of any implementation.

// nonArrayFields returns every scalar and mapping field that complements the
// sequence fields in baseArrayFields.
func nonArrayFields() []FieldSpec {
	var fields []FieldSpec
	fields = append(fields, topLevelFields()...)
	fields = append(fields, serviceScalarFields()...)
	fields = append(fields, buildScalarFields()...)
	fields = append(fields, deployScalarFields()...)
	fields = append(fields, networkScalarFields()...)
	fields = append(fields, volumeScalarFields()...)
	fields = append(fields, configScalarFields()...)
	fields = append(fields, secretScalarFields()...)
	fields = append(fields, includeScalarFields()...)
	fields = append(fields, casaOSScalarFields()...)
	fields = append(fields, nestedScalarFields()...)
	fields = append(fields, mapFields()...)
	return fields
}

// --- value triples ---------------------------------------------------------

func textTriple() [3]any     { return [3]any{"base-value", "user-value", "remote-value"} }
func durationTriple() [3]any { return [3]any{"1s", "2s", "3s"} }
func intTriple() [3]any      { return [3]any{1, 2, 3} }
func floatTriple() [3]any    { return [3]any{0.1, 0.2, 0.3} }
func boolTriple() [3]any     { return twoValue(false, true) }

// twoValue models a field whose authored domain has only two distinct values.
// The third slot repeats the second one so the modification states still move
// away from the base value; the three-slot matrix cannot represent a third
// distinct boolean or two-value enum.
func twoValue(a, b any) [3]any { return [3]any{a, b, b} }

func enumTriple(a, b, c any) [3]any { return [3]any{a, b, c} }

func pairTriples(prefix string) ([3]any, [3]any) {
	return [3]any{prefix + "-a-base", prefix + "-a-user", prefix + "-a-remote"},
		[3]any{prefix + "-b-base", prefix + "-b-user", prefix + "-b-remote"}
}

// --- constructors ----------------------------------------------------------

// atomicField models a scalar leaf as a single whole value. The wrap keeps the
// authored YAML a scalar; the shared renderer otherwise emits the value list.
func atomicField(id, path string, values [3]any) FieldSpec {
	boundary := strings.Split(path, ".")
	return FieldSpec{
		ID: id, Path: boundary, Class: ClassAtomic,
		Values: values, CrossProduct: false, PairSemantics: PairDisabled,
		ResetBoundary: boundary,
		Wrap: func(current []any, present bool) map[string]any {
			if !present {
				return map[string]any{}
			}
			return mappingPath(boundary, current[0])
		},
	}
}

// wrappedScalar models a scalar leaf nested inside an array item. The provided
// wrap rebuilds the surrounding container from the logical values.
func wrappedScalar(id, displayPath string, values [3]any, wrap func([]any, bool) map[string]any, reset []string) FieldSpec {
	return FieldSpec{
		ID: id, Path: strings.Split(displayPath, "."), Class: ClassAtomic,
		Values: values, CrossProduct: false, PairSemantics: PairDisabled,
		Wrap: wrap, ResetBoundary: append([]string(nil), reset...),
	}
}

// mapField models a free-form mapping as named logical items. Each key is one
// logical item; the value of that key is the item value.
func mapField(id, path, keyA, keyB string, a, b [3]any) FieldSpec {
	keyC := keyB + "-c"
	spec := arrayField(id, path, ClassNamedItem, [3][3]any{
		{map[string]any{keyA: a[0]}, map[string]any{keyA: a[1]}, map[string]any{keyA: a[2]}},
		{map[string]any{keyB: b[0]}, map[string]any{keyB: b[1]}, map[string]any{keyB: b[2]}},
		{map[string]any{keyC: b[0]}, map[string]any{keyC: b[1]}, map[string]any{keyC: b[2]}},
	})
	boundary := append([]string(nil), spec.Path...)
	spec.Wrap = func(values []any, present bool) map[string]any {
		if !present {
			return map[string]any{}
		}
		return mappingPath(boundary, mergeNamedItems(values))
	}
	return spec
}

// wrappedMapField models a free-form mapping nested inside an array item.
func wrappedMapField(id, displayPath, keyA, keyB string, a, b [3]any, build func(map[string]any, bool) map[string]any, reset []string) FieldSpec {
	keyC := keyB + "-c"
	spec := arrayField(id, displayPath, ClassNamedItem, [3][3]any{
		{map[string]any{keyA: a[0]}, map[string]any{keyA: a[1]}, map[string]any{keyA: a[2]}},
		{map[string]any{keyB: b[0]}, map[string]any{keyB: b[1]}, map[string]any{keyB: b[2]}},
		{map[string]any{keyC: b[0]}, map[string]any{keyC: b[1]}, map[string]any{keyC: b[2]}},
	})
	spec.Wrap = func(values []any, present bool) map[string]any {
		return build(mergeNamedItems(values), present)
	}
	spec.ResetBoundary = append([]string(nil), reset...)
	return spec
}

func mergeNamedItems(values []any) map[string]any {
	merged := make(map[string]any)
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range item {
			merged[key] = value
		}
	}
	return merged
}

func serviceMapping(field string, value any) map[string]any {
	return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", field: value}}}
}

func setNestedValue(root map[string]any, path []string, value any) {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

// --- field groups ----------------------------------------------------------

func topLevelFields() []FieldSpec {
	return []FieldSpec{
		atomicField("top.name", "name", textTriple()),
		atomicField("top.version", "version", textTriple()),
	}
}

func serviceScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("service.attach", "services.app.attach", boolTriple()),
		atomicField("service.cgroup", "services.app.cgroup", twoValue("host", "private")),
		atomicField("service.cgroup-parent", "services.app.cgroup_parent", textTriple()),
		atomicField("service.container-name", "services.app.container_name", textTriple()),
		atomicField("service.cpu-count", "services.app.cpu_count", intTriple()),
		atomicField("service.cpu-percent", "services.app.cpu_percent", intTriple()),
		atomicField("service.cpu-shares", "services.app.cpu_shares", intTriple()),
		atomicField("service.cpu-quota", "services.app.cpu_quota", intTriple()),
		atomicField("service.cpu-period", "services.app.cpu_period", intTriple()),
		atomicField("service.cpu-rt-period", "services.app.cpu_rt_period", intTriple()),
		atomicField("service.cpu-rt-runtime", "services.app.cpu_rt_runtime", intTriple()),
		atomicField("service.cpus", "services.app.cpus", intTriple()),
		atomicField("service.cpuset", "services.app.cpuset", textTriple()),
		atomicField("service.domainname", "services.app.domainname", textTriple()),
		atomicField("service.image", "services.app.image", textTriple()),
		atomicField("service.hostname", "services.app.hostname", textTriple()),
		atomicField("service.init", "services.app.init", boolTriple()),
		atomicField("service.ipc", "services.app.ipc", textTriple()),
		atomicField("service.isolation", "services.app.isolation", textTriple()),
		atomicField("service.mac-address", "services.app.mac_address", textTriple()),
		atomicField("service.mem-limit", "services.app.mem_limit", intTriple()),
		atomicField("service.mem-reservation", "services.app.mem_reservation", intTriple()),
		atomicField("service.mem-swappiness", "services.app.mem_swappiness", intTriple()),
		atomicField("service.memswap-limit", "services.app.memswap_limit", intTriple()),
		atomicField("service.network-mode", "services.app.network_mode", textTriple()),
		atomicField("service.oom-kill-disable", "services.app.oom_kill_disable", boolTriple()),
		atomicField("service.oom-score-adj", "services.app.oom_score_adj", intTriple()),
		atomicField("service.pid", "services.app.pid", textTriple()),
		atomicField("service.pids-limit", "services.app.pids_limit", intTriple()),
		atomicField("service.platform", "services.app.platform", textTriple()),
		atomicField("service.privileged", "services.app.privileged", boolTriple()),
		atomicField("service.pull-policy", "services.app.pull_policy", enumTriple("always", "never", "missing")),
		atomicField("service.read-only", "services.app.read_only", boolTriple()),
		atomicField("service.restart", "services.app.restart", enumTriple("always", "on-failure", "unless-stopped")),
		atomicField("service.runtime", "services.app.runtime", textTriple()),
		atomicField("service.scale", "services.app.scale", intTriple()),
		atomicField("service.shm-size", "services.app.shm_size", intTriple()),
		atomicField("service.stdin-open", "services.app.stdin_open", boolTriple()),
		atomicField("service.stop-grace-period", "services.app.stop_grace_period", durationTriple()),
		atomicField("service.stop-signal", "services.app.stop_signal", textTriple()),
		atomicField("service.tty", "services.app.tty", boolTriple()),
		atomicField("service.user", "services.app.user", textTriple()),
		atomicField("service.userns-mode", "services.app.userns_mode", textTriple()),
		atomicField("service.uts", "services.app.uts", textTriple()),
		atomicField("service.working-dir", "services.app.working_dir", textTriple()),
		atomicField("service.build", "services.app.build", textTriple()),
		atomicField("service.extends", "services.app.extends", textTriple()),
		atomicField("service.extends.service", "services.app.extends.service", textTriple()),
		atomicField("service.extends.file", "services.app.extends.file", textTriple()),
		atomicField("service.credential-spec.config", "services.app.credential_spec.config", textTriple()),
		atomicField("service.credential-spec.file", "services.app.credential_spec.file", textTriple()),
		atomicField("service.credential-spec.registry", "services.app.credential_spec.registry", textTriple()),
		atomicField("service.healthcheck.disable", "services.app.healthcheck.disable", boolTriple()),
		atomicField("service.healthcheck.interval", "services.app.healthcheck.interval", durationTriple()),
		atomicField("service.healthcheck.retries", "services.app.healthcheck.retries", intTriple()),
		atomicField("service.healthcheck.start-interval", "services.app.healthcheck.start_interval", durationTriple()),
		atomicField("service.healthcheck.start-period", "services.app.healthcheck.start_period", durationTriple()),
		atomicField("service.healthcheck.timeout", "services.app.healthcheck.timeout", durationTriple()),
		atomicField("service.logging.driver", "services.app.logging.driver", textTriple()),
		atomicField("service.network.ipv4-address", "services.app.networks.default.ipv4_address", textTriple()),
		atomicField("service.network.ipv6-address", "services.app.networks.default.ipv6_address", textTriple()),
		atomicField("service.network.mac-address", "services.app.networks.default.mac_address", textTriple()),
		atomicField("service.network.priority", "services.app.networks.default.priority", intTriple()),
		atomicField("blkio.weight", "services.app.blkio_config.weight", intTriple()),
	}
}

func buildScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("build.context", "services.app.build.context", textTriple()),
		atomicField("build.dockerfile", "services.app.build.dockerfile", textTriple()),
		atomicField("build.dockerfile-inline", "services.app.build.dockerfile_inline", textTriple()),
		atomicField("build.network", "services.app.build.network", textTriple()),
		atomicField("build.target", "services.app.build.target", textTriple()),
		atomicField("build.no-cache", "services.app.build.no_cache", boolTriple()),
		atomicField("build.pull", "services.app.build.pull", boolTriple()),
		atomicField("build.isolation", "services.app.build.isolation", textTriple()),
		atomicField("build.privileged", "services.app.build.privileged", boolTriple()),
		atomicField("build.shm-size", "services.app.build.shm_size", intTriple()),
	}
}

func deployScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("deploy.mode", "services.app.deploy.mode", textTriple()),
		atomicField("deploy.endpoint-mode", "services.app.deploy.endpoint_mode", textTriple()),
		atomicField("deploy.replicas", "services.app.deploy.replicas", intTriple()),
		atomicField("deploy.placement.max-replicas-per-node", "services.app.deploy.placement.max_replicas_per_node", intTriple()),
		atomicField("deploy.resources.limits.cpus", "services.app.deploy.resources.limits.cpus", intTriple()),
		atomicField("deploy.resources.limits.memory", "services.app.deploy.resources.limits.memory", textTriple()),
		atomicField("deploy.resources.limits.pids", "services.app.deploy.resources.limits.pids", intTriple()),
		atomicField("deploy.resources.reservations.cpus", "services.app.deploy.resources.reservations.cpus", intTriple()),
		atomicField("deploy.resources.reservations.memory", "services.app.deploy.resources.reservations.memory", textTriple()),
		atomicField("deploy.restart-policy.condition", "services.app.deploy.restart_policy.condition", enumTriple("none", "on-failure", "any")),
		atomicField("deploy.restart-policy.delay", "services.app.deploy.restart_policy.delay", durationTriple()),
		atomicField("deploy.restart-policy.max-attempts", "services.app.deploy.restart_policy.max_attempts", intTriple()),
		atomicField("deploy.restart-policy.window", "services.app.deploy.restart_policy.window", durationTriple()),
		atomicField("deploy.update-config.parallelism", "services.app.deploy.update_config.parallelism", intTriple()),
		atomicField("deploy.update-config.delay", "services.app.deploy.update_config.delay", durationTriple()),
		atomicField("deploy.update-config.failure-action", "services.app.deploy.update_config.failure_action", enumTriple("continue", "rollback", "pause")),
		atomicField("deploy.update-config.monitor", "services.app.deploy.update_config.monitor", durationTriple()),
		atomicField("deploy.update-config.max-failure-ratio", "services.app.deploy.update_config.max_failure_ratio", floatTriple()),
		atomicField("deploy.update-config.order", "services.app.deploy.update_config.order", twoValue("start-first", "stop-first")),
		atomicField("deploy.rollback-config.parallelism", "services.app.deploy.rollback_config.parallelism", intTriple()),
		atomicField("deploy.rollback-config.delay", "services.app.deploy.rollback_config.delay", durationTriple()),
		atomicField("deploy.rollback-config.failure-action", "services.app.deploy.rollback_config.failure_action", enumTriple("continue", "rollback", "pause")),
		atomicField("deploy.rollback-config.monitor", "services.app.deploy.rollback_config.monitor", durationTriple()),
		atomicField("deploy.rollback-config.max-failure-ratio", "services.app.deploy.rollback_config.max_failure_ratio", floatTriple()),
		atomicField("deploy.rollback-config.order", "services.app.deploy.rollback_config.order", twoValue("start-first", "stop-first")),
	}
}

func networkScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("network.name", "networks.default.name", textTriple()),
		atomicField("network.driver", "networks.default.driver", textTriple()),
		atomicField("network.attachable", "networks.default.attachable", boolTriple()),
		atomicField("network.enable-ipv6", "networks.default.enable_ipv6", boolTriple()),
		atomicField("network.external", "networks.default.external", boolTriple()),
		atomicField("network.external.name", "networks.default.external.name", textTriple()),
		atomicField("network.internal", "networks.default.internal", boolTriple()),
		atomicField("network.ipam.driver", "networks.default.ipam.driver", textTriple()),
	}
}

func volumeScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("volume.name", "volumes.default.name", textTriple()),
		atomicField("volume.driver", "volumes.default.driver", textTriple()),
		atomicField("volume.external", "volumes.default.external", boolTriple()),
		atomicField("volume.external.name", "volumes.default.external.name", textTriple()),
	}
}

func configScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("config.name", "configs.default.name", textTriple()),
		atomicField("config.content", "configs.default.content", textTriple()),
		atomicField("config.environment", "configs.default.environment", textTriple()),
		atomicField("config.file", "configs.default.file", textTriple()),
		atomicField("config.external", "configs.default.external", boolTriple()),
		atomicField("config.external.name", "configs.default.external.name", textTriple()),
		atomicField("config.template-driver", "configs.default.template_driver", textTriple()),
	}
}

func secretScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("secret.name", "secrets.default.name", textTriple()),
		atomicField("secret.environment", "secrets.default.environment", textTriple()),
		atomicField("secret.file", "secrets.default.file", textTriple()),
		atomicField("secret.external", "secrets.default.external", boolTriple()),
		atomicField("secret.external.name", "secrets.default.external.name", textTriple()),
		atomicField("secret.driver", "secrets.default.driver", textTriple()),
		atomicField("secret.template-driver", "secrets.default.template_driver", textTriple()),
	}
}

func includeScalarFields() []FieldSpec {
	return []FieldSpec{
		wrappedScalar("include.project-directory", "include[].project_directory", textTriple(), wrapIncludeField("project_directory"), []string{"include"}),
	}
}

func casaOSScalarFields() []FieldSpec {
	return []FieldSpec{
		atomicField("x-casaos.id", "x-casaos.id", textTriple()),
		atomicField("x-casaos.repo-id", "x-casaos.repo_id", textTriple()),
		atomicField("x-casaos.version", "x-casaos.version", textTriple()),
		atomicField("x-casaos.icon", "x-casaos.icon", textTriple()),
		atomicField("x-casaos.release-note", "x-casaos.release_note", textTriple()),
		atomicField("x-casaos.thumbnail", "x-casaos.thumbnail", textTriple()),
		atomicField("x-casaos.author", "x-casaos.author", textTriple()),
		atomicField("x-casaos.developer", "x-casaos.developer", textTriple()),
		atomicField("x-casaos.category", "x-casaos.category", textTriple()),
		atomicField("x-casaos.tips.custom", "x-casaos.tips.custom", textTriple()),
		atomicField("x-casaos.scheme", "x-casaos.scheme", twoValue("http", "https")),
		atomicField("x-casaos.hostname", "x-casaos.hostname", textTriple()),
		atomicField("x-casaos.port-map", "x-casaos.port_map", textTriple()),
		atomicField("x-casaos.index", "x-casaos.index", textTriple()),
		atomicField("x-casaos.main", "x-casaos.main", textTriple()),
		atomicField("x-casaos.autostart", "x-casaos.autostart", boolTriple()),
		atomicField("x-casaos.image-drift-check", "x-casaos.image_drift_check", boolTriple()),
		atomicField("x-casaos.store-app-id", "x-casaos.store_app_id", textTriple()),
		atomicField("x-casaos.is-uncontrolled", "x-casaos.is_uncontrolled", boolTriple()),
	}
}

// nestedScalarFields models leaves that live inside array items already covered
// by the sequence corpus. The item identity fields are provided as fixed
// context by the wrap functions.
func nestedScalarFields() []FieldSpec {
	return []FieldSpec{
		// ports[]: target and published are covered by service.ports.
		wrappedScalar("service.ports.host-ip", "services.app.ports[].host_ip", textTriple(), wrapPortField("host_ip"), []string{"services", "app", "ports"}),
		wrappedScalar("service.ports.mode", "services.app.ports[].mode", twoValue("host", "ingress"), wrapPortField("mode"), []string{"services", "app", "ports"}),
		wrappedScalar("service.ports.protocol", "services.app.ports[].protocol", twoValue("tcp", "udp"), wrapPortField("protocol"), []string{"services", "app", "ports"}),

		// volumes[]: source is covered by service.volumes.
		wrappedScalar("service.volumes.type", "services.app.volumes[].type", twoValue("bind", "volume"), wrapVolumeField("type"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.read-only", "services.app.volumes[].read_only", boolTriple(), wrapVolumeField("read_only"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.consistency", "services.app.volumes[].consistency", textTriple(), wrapVolumeField("consistency"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.bind.propagation", "services.app.volumes[].bind.propagation", textTriple(), wrapVolumeField("bind.propagation"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.bind.create-host-path", "services.app.volumes[].bind.create_host_path", boolTriple(), wrapVolumeField("bind.create_host_path"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.bind.selinux", "services.app.volumes[].bind.selinux", textTriple(), wrapVolumeField("bind.selinux"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.volume.nocopy", "services.app.volumes[].volume.nocopy", boolTriple(), wrapVolumeField("volume.nocopy"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.tmpfs.size", "services.app.volumes[].tmpfs.size", intTriple(), wrapVolumeField("tmpfs.size"), []string{"services", "app", "volumes"}),
		wrappedScalar("service.volumes.tmpfs.mode", "services.app.volumes[].tmpfs.mode", intTriple(), wrapVolumeField("tmpfs.mode"), []string{"services", "app", "volumes"}),

		// configs[] / secrets[]: source and target are covered by the sequence corpus.
		wrappedScalar("service.configs.uid", "services.app.configs[].uid", textTriple(), wrapFileReferenceField("configs", "uid"), []string{"services", "app", "configs"}),
		wrappedScalar("service.configs.gid", "services.app.configs[].gid", textTriple(), wrapFileReferenceField("configs", "gid"), []string{"services", "app", "configs"}),
		wrappedScalar("service.configs.mode", "services.app.configs[].mode", intTriple(), wrapFileReferenceField("configs", "mode"), []string{"services", "app", "configs"}),
		wrappedScalar("service.secrets.uid", "services.app.secrets[].uid", textTriple(), wrapFileReferenceField("secrets", "uid"), []string{"services", "app", "secrets"}),
		wrappedScalar("service.secrets.gid", "services.app.secrets[].gid", textTriple(), wrapFileReferenceField("secrets", "gid"), []string{"services", "app", "secrets"}),
		wrappedScalar("service.secrets.mode", "services.app.secrets[].mode", intTriple(), wrapFileReferenceField("secrets", "mode"), []string{"services", "app", "secrets"}),

		// build.secrets[].
		wrappedScalar("build.secrets.uid", "services.app.build.secrets[].uid", textTriple(), wrapBuildSecretField("uid"), []string{"services", "app", "build", "secrets"}),
		wrappedScalar("build.secrets.gid", "services.app.build.secrets[].gid", textTriple(), wrapBuildSecretField("gid"), []string{"services", "app", "build", "secrets"}),
		wrappedScalar("build.secrets.mode", "services.app.build.secrets[].mode", intTriple(), wrapBuildSecretField("mode"), []string{"services", "app", "build", "secrets"}),

		// deploy devices[]: driver is covered by the sequence corpus.
		wrappedScalar("deploy.resources.limits.device.count", "services.app.deploy.resources.limits.devices[].count", intTriple(), wrapDeviceRequestField("limits", "count"), []string{"services", "app", "deploy", "resources", "limits", "devices"}),
		wrappedScalar("deploy.resources.reservations.device.count", "services.app.deploy.resources.reservations.devices[].count", intTriple(), wrapDeviceRequestField("reservations", "count"), []string{"services", "app", "deploy", "resources", "reservations", "devices"}),

		// generic_resources[]: value is covered by the sequence corpus.
		wrappedScalar("deploy.resources.limits.generic-resource.kind", "services.app.deploy.resources.limits.generic_resources[].discrete_resource_spec.kind", textTriple(), wrapGenericResourceKind("limits"), []string{"services", "app", "deploy", "resources", "limits", "generic_resources"}),
		wrappedScalar("deploy.resources.reservations.generic-resource.kind", "services.app.deploy.resources.reservations.generic_resources[].discrete_resource_spec.kind", textTriple(), wrapGenericResourceKind("reservations"), []string{"services", "app", "deploy", "resources", "reservations", "generic_resources"}),

		// network ipam.config[]: subnet is covered by network.ipam.config.
		wrappedScalar("network.ipam.config.ip-range", "networks.default.ipam.config[].ip_range", textTriple(), wrapIpamConfigField("ip_range"), []string{"networks", "default", "ipam", "config"}),
		wrappedScalar("network.ipam.config.gateway", "networks.default.ipam.config[].gateway", textTriple(), wrapIpamConfigField("gateway"), []string{"networks", "default", "ipam", "config"}),

		// develop.watch[]: path is covered by develop.watch.
		wrappedScalar("develop.watch.action", "services.app.develop.watch[].action", enumTriple("rebuild", "sync", "sync+restart"), wrapDevelopWatchField("action"), []string{"services", "app", "develop", "watch"}),
		wrappedScalar("develop.watch.target", "services.app.develop.watch[].target", textTriple(), wrapDevelopWatchField("target"), []string{"services", "app", "develop", "watch"}),

		// depends_on[name] attributes in the long mapping form.
		wrappedScalar("service.depends-on.condition", "services.app.depends_on.default.condition", enumTriple("service_started", "service_healthy", "service_completed_successfully"), wrapDependsOnField("condition"), []string{"services", "app", "depends_on"}),
		wrappedScalar("service.depends-on.required", "services.app.depends_on.default.required", boolTriple(), wrapDependsOnField("required"), []string{"services", "app", "depends_on"}),
		wrappedScalar("service.depends-on.restart", "services.app.depends_on.default.restart", boolTriple(), wrapDependsOnField("restart"), []string{"services", "app", "depends_on"}),
	}
}

// --- map groups ------------------------------------------------------------

func mapFields() []FieldSpec {
	loggingA, loggingB := pairTriples("max-size")
	storageA, storageB := pairTriples("size")
	buildUlimitA, buildUlimitB := pairTriples("nofile")
	networkOptA, networkOptB := pairTriples("com.docker.network")
	ipamOptA, ipamOptB := pairTriples("ipam-option")
	volumeOptA, volumeOptB := pairTriples("volume-option")
	secretOptA, secretOptB := pairTriples("secret-option")
	titleA, titleB := pairTriples("title")
	imageA, imageB := pairTriples("image")
	descriptionA, descriptionB := pairTriples("description")
	taglineA, taglineB := pairTriples("tagline")
	tipA, tipB := pairTriples("before-install")

	ipamAuxA, ipamAuxB := pairTriples("aux")
	deviceOptionA, deviceOptionB := pairTriples("device-option")

	return []FieldSpec{
		mapField("service.logging.options", "services.app.logging.options", "max-size", "max-file", loggingA, loggingB),
		mapField("service.storage-opt", "services.app.storage_opt", "size", "driver", storageA, storageB),
		mapField("service.ulimits", "services.app.ulimits", "nofile", "nproc",
			[3]any{100, 200, 300}, [3]any{map[string]any{"soft": 100, "hard": 200}, map[string]any{"soft": 200, "hard": 300}, map[string]any{"soft": 300, "hard": 400}}),
		mapField("build.ulimits", "services.app.build.ulimits", "nofile", "nproc", buildUlimitA, buildUlimitB),
		mapField("network.driver-opts", "networks.default.driver_opts", "foo", "bar", networkOptA, networkOptB),
		mapField("network.ipam.options", "networks.default.ipam.options", "a", "b", ipamOptA, ipamOptB),
		mapField("volume.driver-opts", "volumes.default.driver_opts", "type", "device", volumeOptA, volumeOptB),
		mapField("secret.driver-opts", "secrets.default.driver_opts", "a", "b", secretOptA, secretOptB),
		mapField("x-casaos.title", "x-casaos.title", "en", "zh-CN", titleA, titleB),
		mapField("x-casaos.image", "x-casaos.image", "en", "zh-CN", imageA, imageB),
		mapField("x-casaos.description", "x-casaos.description", "en", "zh-CN", descriptionA, descriptionB),
		mapField("x-casaos.tagline", "x-casaos.tagline", "en", "zh-CN", taglineA, taglineB),
		mapField("x-casaos.tips.before-install", "x-casaos.tips.before_install", "en", "zh-CN", tipA, tipB),

		// deploy.labels also accepts the list form; the base entry models the
		// list and variants_maps.go registers the mapping spelling.
		arrayField("deploy.labels-list", "services.app.deploy.labels", ClassNamedItem, keyValueValues("deploy.label")),

		wrappedMapField("network.ipam.config.aux-addresses", "networks.default.ipam.config[].aux_addresses", "host-a", "host-b", ipamAuxA, ipamAuxB, buildIpamAuxAddresses, []string{"networks", "default", "ipam", "config"}),
		wrappedMapField("deploy.resources.limits.device.options", "services.app.deploy.resources.limits.devices[].options", "a", "b", deviceOptionA, deviceOptionB, buildDeviceOptions("limits"), []string{"services", "app", "deploy", "resources", "limits", "devices"}),
		wrappedMapField("deploy.resources.reservations.device.options", "services.app.deploy.resources.reservations.devices[].options", "a", "b", deviceOptionA, deviceOptionB, buildDeviceOptions("reservations"), []string{"services", "app", "deploy", "resources", "reservations", "devices"}),
	}
}
