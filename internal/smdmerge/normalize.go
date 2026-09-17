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
	for _, field := range []string{"environment", "labels"} {
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
	for _, field := range []string{"env_file", "cap_add", "cap_drop", "dns", "dns_opt", "dns_search", "expose", "profiles", "tmpfs"} {
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
	return nil
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
		result = append(result, map[string]any{
			"type": volume.Type, "source": volume.Source, "target": volume.Target,
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
			result = append(result, map[string]any{"source": item, "target": item})
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
			result = append(result, entry)
		default:
			return nil, fmt.Errorf("unsupported reference item %T", raw)
		}
	}
	return result, nil
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
