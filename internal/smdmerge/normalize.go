package smdmerge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	composetypes "github.com/compose-spec/compose-go/types"
	"gopkg.in/yaml.v3"
)

func normalizeYAML(content []byte) (map[string]any, error) {
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	for _, resourceType := range []string{"networks", "volumes", "configs", "secrets"} {
		resources, _ := document[resourceType].(map[string]any)
		for resourceName, raw := range resources {
			resource, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if value, exists := resource["labels"]; exists {
				normalized, err := normalizeKeyValues(value, false)
				if err != nil {
					return nil, fmt.Errorf("%s %q labels: %w", resourceType, resourceName, err)
				}
				resource["labels"] = normalized
			}
			if resourceType == "networks" {
				if err := normalizeNetworkIpam(resource); err != nil {
					return nil, fmt.Errorf("network %q ipam: %w", resourceName, err)
				}
			}
		}
	}
	services, _ := document["services"].(map[string]any)
	for serviceName, raw := range services {
		service, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if err := normalizeService(service); err != nil {
			return nil, fmt.Errorf("service %q: %w", serviceName, err)
		}
	}
	return document, nil
}

func normalizeService(service map[string]any) error {
	for _, field := range []string{"annotations", "environment", "extra_hosts", "labels", "sysctls"} {
		if value, exists := service[field]; exists {
			normalized, err := normalizeKeyValues(value, field == "environment")
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			service[field] = normalized
		}
	}
	for _, field := range []string{"depends_on", "networks"} {
		if value, exists := service[field]; exists {
			normalized, err := normalizeNamedMap(value, field == "depends_on")
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			service[field] = normalized
		}
	}
	if networks, ok := service["networks"].(map[string]any); ok {
		for _, raw := range networks {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"aliases", "link_local_ips"} {
				if err := normalizeTokenStringSet(entry, key); err != nil {
					return fmt.Errorf("networks.%s: %w", key, err)
				}
			}
		}
	}
	for _, field := range []string{"env_file", "cap_add", "cap_drop", "dns", "dns_opt", "dns_search", "expose", "profiles", "tmpfs", "device_cgroup_rules", "external_links", "group_add", "links", "security_opt", "volumes_from"} {
		if value, exists := service[field]; exists {
			normalized, err := normalizeStringList(value)
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			service[field] = normalized
		}
	}
	if value, exists := service["ports"]; exists {
		ports, err := normalizePorts(value)
		if err != nil {
			return fmt.Errorf("ports: %w", err)
		}
		service["ports"] = ports
	}
	if value, exists := service["volumes"]; exists {
		volumes, err := normalizeVolumes(value)
		if err != nil {
			return fmt.Errorf("volumes: %w", err)
		}
		service["volumes"] = volumes
	}
	if value, exists := service["devices"]; exists {
		devices, err := normalizeDevices(value)
		if err != nil {
			return fmt.Errorf("devices: %w", err)
		}
		service["devices"] = devices
	}
	for _, field := range []string{"configs", "secrets"} {
		if value, exists := service[field]; exists {
			references, err := normalizeFileReferences(value)
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			service[field] = references
		}
	}
	if raw, ok := service["build"].(map[string]any); ok {
		if err := normalizeBuild(raw); err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}
	if raw, ok := service["blkio_config"].(map[string]any); ok {
		if err := normalizeBlkioConfig(raw); err != nil {
			return fmt.Errorf("blkio_config: %w", err)
		}
	}
	if raw, ok := service["deploy"].(map[string]any); ok {
		if err := normalizeDeploy(raw); err != nil {
			return fmt.Errorf("deploy: %w", err)
		}
	}
	return nil
}

func normalizeBuild(build map[string]any) error {
	for _, field := range []string{"args", "labels", "additional_contexts", "extra_hosts"} {
		if value, exists := build[field]; exists {
			normalized, err := normalizeKeyValues(value, false)
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			build[field] = normalized
		}
	}
	if value, exists := build["secrets"]; exists {
		normalized, err := normalizeFileReferences(value)
		if err != nil {
			return fmt.Errorf("secrets: %w", err)
		}
		build["secrets"] = normalized
	}
	if value, exists := build["ssh"]; exists {
		normalized, err := normalizeStringList(value)
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}
		build["ssh"] = normalized
	}
	return nil
}

func normalizeBlkioConfig(config map[string]any) error {
	for _, field := range []string{"weight_device", "device_read_bps", "device_read_iops", "device_write_bps", "device_write_iops"} {
		if value, exists := config[field]; exists {
			if _, err := normalizePathItems(value); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
	}
	return nil
}

func normalizePathItems(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected mapping item")
		}
		if _, ok := item["path"].(string); !ok {
			return nil, fmt.Errorf("item path must be a string")
		}
	}
	return items, nil
}

func normalizeDeploy(deploy map[string]any) error {
	resources, _ := deploy["resources"].(map[string]any)
	for _, branch := range []string{"limits", "reservations"} {
		limits, _ := resources[branch].(map[string]any)
		if value, exists := limits["generic_resources"]; exists {
			normalized, err := normalizeGenericResources(value)
			if err != nil {
				return fmt.Errorf("resources.%s.generic_resources: %w", branch, err)
			}
			limits["generic_resources"] = normalized
		}
		if value, exists := limits["devices"]; exists {
			if err := normalizeDeployDevices(value); err != nil {
				return fmt.Errorf("resources.%s.devices: %w", branch, err)
			}
		}
	}
	return nil
}

func normalizeDeployDevices(value any) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("expected sequence")
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("expected mapping item")
		}
		driver := fmt.Sprint(item["driver"])
		if token := stableStringOriginToken(driver); token != "" {
			item["__merge_id"] = "token:" + token
		} else {
			item["__merge_id"] = "added:" + driver
		}
		for _, key := range []string{"capabilities", "device_ids"} {
			if err := normalizeTokenStringSet(item, key); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	}
	return nil
}

// normalizeTokenStringSet rewrites a scalar string sequence into the StringSet
// internal representation. Elements sharing a stable origin token are given the
// same identity so a modification is anchored to its logical item.
func normalizeTokenStringSet(container map[string]any, field string) error {
	raw, exists := container[field]
	if !exists {
		return nil
	}
	items, err := normalizeStringList(raw)
	if err != nil {
		return err
	}
	wrapped := make([]any, len(items))
	for index, item := range items {
		value := item.(string)
		id := ""
		if token := stableStringOriginToken(value); token != "" {
			id = "token:" + token
		} else {
			id = "added:" + value
		}
		wrapped[index] = map[string]any{"__value": value, "__merge_id": id}
	}
	container[field] = wrapped
	return nil
}

func normalizeGenericResources(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected mapping item")
		}
		var kind any
		for _, specName := range []string{"discrete_resource_spec", "named_resource_spec"} {
			if spec, ok := item[specName].(map[string]any); ok {
				kind = spec["kind"]
				if kind != nil {
					break
				}
			}
		}
		if kind == nil || fmt.Sprint(kind) == "" {
			return nil, fmt.Errorf("item must contain a resource spec kind")
		}
		item["__merge_id"] = fmt.Sprint(kind)
	}
	return items, nil
}

func normalizeKeyValues(value any, nullWithoutValue bool) (map[string]any, error) {
	if mapping, ok := value.(map[string]any); ok {
		return mapping, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected mapping or sequence")
	}
	result := make(map[string]any, len(items))
	for _, raw := range items {
		item, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string item")
		}
		key, itemValue, found := strings.Cut(item, "=")
		if !found && nullWithoutValue {
			result[key] = nil
		} else if !found {
			result[key] = ""
		} else {
			result[key] = itemValue
		}
	}
	return result, nil
}

func normalizeNamedMap(value any, dependsOn bool) (map[string]any, error) {
	if mapping, ok := value.(map[string]any); ok {
		for name, raw := range mapping {
			if raw == nil {
				mapping[name] = map[string]any{"__present": true}
				continue
			}
			entry, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("entry %q must be a mapping", name)
			}
			entry["__present"] = true
		}
		return mapping, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected mapping or sequence")
	}
	result := make(map[string]any, len(items))
	for _, raw := range items {
		name, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string item")
		}
		entry := map[string]any{"__present": true}
		if dependsOn {
			entry["condition"] = "service_started"
			entry["required"] = true
		}
		result[name] = entry
	}
	return result, nil
}

func normalizeStringList(value any) ([]any, error) {
	if scalar, ok := value.(string); ok {
		return []any{scalar}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected string or sequence")
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return nil, fmt.Errorf("expected string item")
		}
	}
	return items, nil
}

func assignStringSetMergeIDs(baseOld, user, baseNew map[string]any) error {
	serviceNames := map[string]struct{}{}
	for _, document := range []map[string]any{baseOld, user, baseNew} {
		for name := range serviceMap(document) {
			serviceNames[name] = struct{}{}
		}
	}
	fields := []string{"cap_add", "cap_drop", "dns", "dns_opt", "dns_search", "expose", "profiles", "tmpfs", "device_cgroup_rules", "external_links", "group_add", "links", "security_opt", "volumes_from"}
	for serviceName := range serviceNames {
		for _, field := range fields {
			base := serviceStringItems(baseOld, serviceName, field)
			userItems := serviceStringItems(user, serviceName, field)
			target := serviceStringItems(baseNew, serviceName, field)
			correlateStringItems(base, userItems)
			correlateStringItems(base, target)
		}
	}
	for _, document := range []map[string]any{baseOld, user, baseNew} {
		services, _ := document["services"].(map[string]any)
		for _, raw := range services {
			service, _ := raw.(map[string]any)
			if build, ok := service["build"].(map[string]any); ok {
				wrapStringItems(build, "ssh")
			}
		}
		if casa, ok := document["x-casaos"].(map[string]any); ok {
			wrapStringItems(casa, "architectures")
		}
	}
	correlateNestedStringItems(baseOld, user, baseNew, []string{"services", "app", "build", "ssh"})
	correlateNestedStringItems(baseOld, user, baseNew, []string{"x-casaos", "architectures"})
	return nil
}

// normalizeNetworkIpam assigns positional identities to IPAM config entries,
// which carry no stable cross-version attribute in the authored form.
func normalizeNetworkIpam(resource map[string]any) error {
	ipam, ok := resource["ipam"].(map[string]any)
	if !ok {
		return nil
	}
	config, ok := ipam["config"].([]any)
	if !ok {
		return nil
	}
	for index, raw := range config {
		item, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("config item must be a mapping")
		}
		item["__merge_id"] = "pos:" + strconv.Itoa(index)
	}
	return nil
}

func wrapStringItems(container map[string]any, field string) {
	raw, _ := container[field].([]any)
	for index, value := range raw {
		if scalar, ok := value.(string); ok {
			raw[index] = map[string]any{"__value": scalar}
		}
	}
}

func correlateNestedStringItems(baseOld, user, baseNew map[string]any, path []string) {
	base := nestedStringItems(baseOld, path)
	userItems := nestedStringItems(user, path)
	target := nestedStringItems(baseNew, path)
	correlateStringItems(base, userItems)
	correlateStringItems(base, target)
}

func nestedStringItems(document map[string]any, path []string) []map[string]any {
	var current any = document
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapping[key]
	}
	raw, _ := current.([]any)
	items := make([]map[string]any, 0, len(raw))
	for index, value := range raw {
		switch item := value.(type) {
		case string:
			wrapped := map[string]any{"__value": item}
			raw[index] = wrapped
			items = append(items, wrapped)
		case map[string]any:
			items = append(items, item)
		}
	}
	return items
}

func correlateStringItems(base, side []map[string]any) {
	for index, item := range base {
		item["__merge_id"] = "base:" + strconv.Itoa(index)
	}
	used := make(map[string]bool, len(base))

	// Pass 1: identical value belongs to the same logical item.
	baseByValue := make(map[string][]map[string]any, len(base))
	for _, item := range base {
		value := fmt.Sprint(item["__value"])
		baseByValue[value] = append(baseByValue[value], item)
	}
	for _, item := range side {
		value := fmt.Sprint(item["__value"])
		for _, candidate := range baseByValue[value] {
			id := fmt.Sprint(candidate["__merge_id"])
			if used[id] {
				continue
			}
			item["__merge_id"] = id
			used[id] = true
			break
		}
	}

	// Pass 2: a stable origin token identifies the same logical item across
	// versions even when its value changed. This keeps modifications anchored
	// to their original element and lets deletes shift independent entries.
	baseByToken := make(map[string][]map[string]any, len(base))
	for _, item := range base {
		token := stableStringOriginToken(fmt.Sprint(item["__value"]))
		if token == "" {
			continue
		}
		baseByToken[token] = append(baseByToken[token], item)
	}
	for _, item := range side {
		if _, assigned := item["__merge_id"]; assigned {
			continue
		}
		token := stableStringOriginToken(fmt.Sprint(item["__value"]))
		if token == "" {
			continue
		}
		for _, candidate := range baseByToken[token] {
			id := fmt.Sprint(candidate["__merge_id"])
			if used[id] {
				continue
			}
			item["__merge_id"] = id
			used[id] = true
			break
		}
	}

	// Pass 3: when no stable origin token exists anywhere, fall back to
	// positional alignment against unused base entries. This is only safe for
	// tokenless values; with tokens, correlating each side independently by
	// position produces inconsistent ids across user and upstream.
	if len(baseByToken) == 0 {
		var remainingBase []map[string]any
		for _, item := range base {
			id := fmt.Sprint(item["__merge_id"])
			if !used[id] {
				remainingBase = append(remainingBase, item)
			}
		}
		remainingIndex := 0
		for _, item := range side {
			if _, assigned := item["__merge_id"]; assigned {
				continue
			}
			if stableStringOriginToken(fmt.Sprint(item["__value"])) != "" {
				continue
			}
			if remainingIndex < len(remainingBase) {
				item["__merge_id"] = remainingBase[remainingIndex]["__merge_id"]
				remainingIndex++
			}
		}
	}

	// Pass 4: no base anchor. A side element that cannot be matched to base is
	// given a deterministic id shared with the other side, so the two sides
	// resolve the same logical item as a modification (user wins) while
	// distinct tokens remain independent additions.
	for _, item := range side {
		if _, assigned := item["__merge_id"]; assigned {
			continue
		}
		value := fmt.Sprint(item["__value"])
		if token := stableStringOriginToken(value); token != "" {
			item["__merge_id"] = "token:" + token
		} else {
			item["__merge_id"] = "added:" + value
		}
	}
}

// stableStringOriginToken mirrors override.stableOriginToken: a unique trailing
// token after the last dash is treated as the stable identity of a logical item.
func stableStringOriginToken(value string) string {
	index := strings.LastIndexByte(value, '-')
	if index < 0 || index == len(value)-1 {
		return ""
	}
	return value[index+1:]
}

func serviceStringItems(document map[string]any, serviceName, field string) []map[string]any {
	service, _ := serviceMap(document)[serviceName].(map[string]any)
	raw, _ := service[field].([]any)
	items := make([]map[string]any, 0, len(raw))
	for index, value := range raw {
		switch item := value.(type) {
		case string:
			wrapped := map[string]any{"__value": item}
			raw[index] = wrapped
			items = append(items, wrapped)
		case map[string]any:
			items = append(items, item)
		}
	}
	return items
}

func normalizePorts(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	result := make([]any, 0, len(items))
	for _, raw := range items {
		var ports []composetypes.ServicePortConfig
		switch item := raw.(type) {
		case string:
			parsed, err := composetypes.ParsePortConfig(item)
			if err != nil || len(parsed) != 1 {
				return nil, fmt.Errorf("unsupported port %q", item)
			}
			ports = parsed
		case map[string]any:
			content, err := yaml.Marshal(item)
			if err != nil {
				return nil, err
			}
			var port composetypes.ServicePortConfig
			if err := yaml.Unmarshal(content, &port); err != nil {
				return nil, err
			}
			ports = []composetypes.ServicePortConfig{port}
		default:
			return nil, fmt.Errorf("unsupported port item %T", raw)
		}
		port := ports[0]
		if port.Protocol == "" {
			port.Protocol = "tcp"
		}
		if port.Mode == "" {
			port.Mode = "ingress"
		}
		result = append(result, map[string]any{
			"host_ip": port.HostIP, "target": int64(port.Target), "published": port.Published,
			"protocol": port.Protocol, "mode": port.Mode,
		})
	}
	return result, nil
}

func normalizeVolumes(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	result := make([]any, 0, len(items))
	for _, raw := range items {
		var volume composetypes.ServiceVolumeConfig
		switch item := raw.(type) {
		case string:
			parsed, err := loader.ParseVolume(item)
			if err != nil {
				return nil, err
			}
			volume = parsed
		case map[string]any:
			content, err := yaml.Marshal(item)
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(content, &volume); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported volume item %T", raw)
		}
		volumeType := volume.Type
		if volumeType == "" {
			volumeType = composetypes.VolumeTypeVolume
			if strings.HasPrefix(volume.Source, "/") || strings.HasPrefix(volume.Source, ".") || strings.HasPrefix(volume.Source, "~") {
				volumeType = composetypes.VolumeTypeBind
			}
		}
		result = append(result, map[string]any{
			"type": volumeType, "source": volume.Source, "target": volume.Target,
			"read_only": volume.ReadOnly, "consistency": volume.Consistency,
		})
	}
	return result, nil
}

func normalizeDevices(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	result := make([]any, 0, len(items))
	for _, raw := range items {
		var entry map[string]any
		switch item := raw.(type) {
		case string:
			parts := strings.Split(item, ":")
			if len(parts) < 2 || len(parts) > 3 {
				return nil, fmt.Errorf("unsupported device %q", item)
			}
			entry = map[string]any{"source": parts[0], "target": parts[1], "permissions": "rwm"}
			if len(parts) == 3 {
				entry["permissions"] = parts[2]
			}
		case map[string]any:
			entry = item
		default:
			return nil, fmt.Errorf("unsupported device item %T", raw)
		}
		if _, exists := entry["permissions"]; !exists {
			entry["permissions"] = "rwm"
		}
		result = append(result, entry)
	}
	return result, nil
}

func normalizeFileReferences(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}
	result := make([]any, 0, len(items))
	for _, raw := range items {
		switch item := raw.(type) {
		case string:
			result = append(result, map[string]any{
				"source": item, "target": item, "__merge_id": fileReferenceMergeID(item, item),
			})
		case map[string]any:
			entry := make(map[string]any, len(item)+1)
			for key, value := range item {
				entry[key] = value
			}
			if _, exists := entry["target"]; !exists {
				entry["target"] = fmt.Sprint(entry["source"])
			}
			if mode, exists := entry["mode"].(int); exists {
				entry["mode"] = int64(mode)
			}
			entry["__merge_id"] = fileReferenceMergeID(fmt.Sprint(entry["source"]), fmt.Sprint(entry["target"]))
			result = append(result, entry)
		default:
			return nil, fmt.Errorf("unsupported reference item %T", raw)
		}
	}
	return result, nil
}

// fileReferenceMergeID derives the merge identity of a config/secret reference.
// An explicit target distinct from the source is the stable identity. Short
// syntax carries only the source, so a stable origin token is used instead;
// this keeps a renamed reference aligned with its original logical item.
func fileReferenceMergeID(source, target string) string {
	if target != "" && target != source {
		return "target:" + target
	}
	if token := stableStringOriginToken(source); token != "" {
		return "token:" + token
	}
	if source != "" {
		return "source:" + source
	}
	return "added:reference"
}

func portExactKey(port map[string]any) string {
	return strings.Join([]string{
		fmt.Sprint(port["host_ip"]), fmt.Sprint(port["published"]),
		fmt.Sprint(port["target"]), fmt.Sprint(port["protocol"]),
	}, "\x00")
}

func portCorrelationKey(port map[string]any) string {
	return strings.Join([]string{
		fmt.Sprint(port["host_ip"]), fmt.Sprint(port["target"]), fmt.Sprint(port["protocol"]),
	}, "\x00")
}

func assignPortMergeIDs(baseOld, user, baseNew map[string]any) error {
	serviceNames := map[string]struct{}{}
	for _, document := range []map[string]any{baseOld, user, baseNew} {
		for name := range serviceMap(document) {
			serviceNames[name] = struct{}{}
		}
	}
	for serviceName := range serviceNames {
		basePorts := servicePorts(baseOld, serviceName)
		userPorts := servicePorts(user, serviceName)
		newPorts := servicePorts(baseNew, serviceName)
		if err := correlatePorts(serviceName, basePorts, userPorts, newPorts); err != nil {
			return err
		}
	}
	return nil
}

func correlatePorts(service string, base, user, target []map[string]any) error {
	baseExact := indexUniquePorts(base, portExactKey)
	for index, port := range base {
		port["__merge_id"] = "base:" + strconv.Itoa(index)
	}
	for _, side := range [][]map[string]any{user, target} {
		for _, port := range side {
			if basePort, ok := baseExact[portExactKey(port)]; ok {
				port["__merge_id"] = basePort["__merge_id"]
			}
		}
	}
	baseCorrelation, baseAmbiguous := indexPorts(base, portCorrelationKey)
	for _, side := range [][]map[string]any{user, target} {
		_, sideAmbiguous := indexPorts(side, portCorrelationKey)
		for _, port := range side {
			if _, assigned := port["__merge_id"]; assigned {
				continue
			}
			key := portCorrelationKey(port)
			if baseAmbiguous[key] || sideAmbiguous[key] {
				return fmt.Errorf("service %q has ambiguous port identity %q", service, key)
			}
			if basePort, exists := baseCorrelation[key]; exists {
				port["__merge_id"] = basePort["__merge_id"]
				continue
			}
			port["__merge_id"] = "added:" + key
		}
	}
	return nil
}

func indexUniquePorts(ports []map[string]any, key func(map[string]any) string) map[string]map[string]any {
	indexed, ambiguous := indexPorts(ports, key)
	for value := range ambiguous {
		delete(indexed, value)
	}
	return indexed
}

func indexPorts(ports []map[string]any, key func(map[string]any) string) (map[string]map[string]any, map[string]bool) {
	indexed := make(map[string]map[string]any, len(ports))
	ambiguous := make(map[string]bool)
	for _, port := range ports {
		value := key(port)
		if _, exists := indexed[value]; exists {
			ambiguous[value] = true
		}
		indexed[value] = port
	}
	return indexed, ambiguous
}

func serviceMap(document map[string]any) map[string]any {
	services, _ := document["services"].(map[string]any)
	return services
}

func servicePorts(document map[string]any, serviceName string) []map[string]any {
	service, _ := serviceMap(document)[serviceName].(map[string]any)
	raw, _ := service["ports"].([]any)
	ports := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if port, ok := item.(map[string]any); ok {
			ports = append(ports, port)
		}
	}
	return ports
}
