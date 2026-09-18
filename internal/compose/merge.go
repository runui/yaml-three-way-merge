package compose

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	composetypes "github.com/compose-spec/compose-go/types"
	"gopkg.in/yaml.v3"
)

const (
	resetTag    = "!reset"
	overrideTag = "!override"
)

// MergeYAML applies Compose override rules without interpreting or validating
// Compose fields. Docker Compose remains the authority for the resulting file.
func MergeYAML(baseYAML, overrideYAML []byte) ([]byte, error) {
	base, err := ParseMapping(baseYAML)
	if err != nil {
		return nil, fmt.Errorf("parse base YAML: %w", err)
	}
	base, err = materializeDocument(base)
	if err != nil {
		return nil, fmt.Errorf("resolve base YAML: %w", err)
	}
	if len(overrideYAML) == 0 {
		return bytes.Clone(baseYAML), nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(overrideYAML))
	for {
		var override yaml.Node
		if err := decoder.Decode(&override); err != nil {
			if err == io.EOF {
				break
			}
			return nil, &OverrideError{Err: fmt.Errorf("parse override YAML: %w", err)}
		}
		if len(override.Content) == 0 {
			continue
		}
		if len(override.Content) != 1 || override.Content[0].Kind != yaml.MappingNode {
			return nil, &OverrideError{Err: fmt.Errorf("override Compose YAML must be a mapping")}
		}
		resolved, err := materializeDocument(&override)
		if err != nil {
			return nil, &OverrideError{Err: fmt.Errorf("resolve override YAML: %w", err)}
		}
		mergeMapping(base.Content[0], resolved.Content[0], nil)
	}
	return yaml.Marshal(base)
}

func mergeMapping(base, override *yaml.Node, path []string) {
	for index := 0; index+1 < len(override.Content); index += 2 {
		key, value := override.Content[index], override.Content[index+1]
		childPath := appendPath(path, key.Value)
		baseIndex := mappingIndex(base, key.Value)
		switch value.Tag {
		case resetTag:
			if baseIndex >= 0 {
				base.Content = append(base.Content[:baseIndex], base.Content[baseIndex+2:]...)
			}
			continue
		case overrideTag:
			replacement := CloneNode(value)
			normalizeDirectiveTag(replacement)
			setMappingValue(base, baseIndex, key, replacement)
			continue
		}
		if baseIndex < 0 {
			setMappingValue(base, baseIndex, key, CloneNode(value))
			continue
		}
		baseValue := base.Content[baseIndex+1]
		base.Content[baseIndex+1] = mergeNode(baseValue, value, childPath)
	}
}

func mergeNode(base, override *yaml.Node, path []string) *yaml.Node {
	if replaceComposeValue(path) {
		return CloneNode(override)
	}
	if merged, ok := mergeComposeSpecial(base, override, path); ok {
		return merged
	}
	if base.Kind == yaml.MappingNode && override.Kind == yaml.MappingNode {
		merged := CloneNode(base)
		mergeMapping(merged, override, path)
		return merged
	}
	if base.Kind == yaml.SequenceNode && override.Kind == yaml.SequenceNode {
		merged := CloneNode(base)
		for _, value := range override.Content {
			appendUniqueComposeValue(merged, CloneNode(value), path)
		}
		return merged
	}
	return CloneNode(override)
}

func mergeComposeSpecial(base, override *yaml.Node, path []string) (*yaml.Node, bool) {
	if len(path) != 3 || path[0] != "services" {
		return nil, false
	}
	switch path[2] {
	case "environment", "labels", "annotations", "sysctls", "extra_hosts":
		baseMapping, baseOK := keyValueMapping(base, path[2] == "environment")
		overrideMapping, overrideOK := keyValueMapping(override, path[2] == "environment")
		if baseOK && overrideOK {
			mergeMapping(baseMapping, overrideMapping, path)
			return baseMapping, true
		}
	case "env_file", "dns", "dns_opt", "dns_search", "tmpfs":
		return mergeScalarOrSequence(base, override, path), true
	case "build":
		baseMapping, baseOK := buildMapping(base)
		overrideMapping, overrideOK := buildMapping(override)
		if baseOK && overrideOK {
			mergeMapping(baseMapping, overrideMapping, path)
			return baseMapping, true
		}
	case "depends_on", "networks":
		baseMapping, baseOK := nameMapping(base, path[2] == "depends_on")
		overrideMapping, overrideOK := nameMapping(override, path[2] == "depends_on")
		if baseOK && overrideOK {
			mergeMapping(baseMapping, overrideMapping, path)
			return baseMapping, true
		}
	case "logging":
		baseDriver, _ := mappingScalar(base, "driver")
		overrideDriver, _ := mappingScalar(override, "driver")
		if baseDriver != "" && overrideDriver != "" && baseDriver != overrideDriver {
			return CloneNode(override), true
		}
	}
	return nil, false
}

func keyValueMapping(node *yaml.Node, nullWithoutValue bool) (*yaml.Node, bool) {
	if node.Kind == yaml.MappingNode {
		return CloneNode(node), true
	}
	if node.Kind != yaml.SequenceNode {
		return nil, false
	}
	result := mappingNode()
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, false
		}
		key, value, found := strings.Cut(item.Value, "=")
		if !found {
			if nullWithoutValue {
				setMappingValue(result, mappingIndex(result, key), scalarNode(key), &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"})
				continue
			}
			value = ""
		}
		setMappingValue(result, mappingIndex(result, key), scalarNode(key), scalarNode(value))
	}
	return result, true
}

func mergeScalarOrSequence(base, override *yaml.Node, path []string) *yaml.Node {
	result := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, node := range []*yaml.Node{base, override} {
		if node.Kind == yaml.SequenceNode {
			for _, item := range node.Content {
				appendUniqueComposeValue(result, CloneNode(item), path)
			}
			continue
		}
		appendUniqueComposeValue(result, CloneNode(node), path)
	}
	return result
}

func buildMapping(node *yaml.Node) (*yaml.Node, bool) {
	if node.Kind == yaml.MappingNode {
		return CloneNode(node), true
	}
	if node.Kind != yaml.ScalarNode {
		return nil, false
	}
	result := mappingNode()
	setMappingValue(result, -1, scalarNode("context"), CloneNode(node))
	return result, true
}

func nameMapping(node *yaml.Node, dependsOn bool) (*yaml.Node, bool) {
	if node.Kind == yaml.MappingNode {
		return CloneNode(node), true
	}
	if node.Kind != yaml.SequenceNode {
		return nil, false
	}
	result := mappingNode()
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, false
		}
		value := mappingNode()
		if dependsOn {
			setMappingValue(value, -1, scalarNode("condition"), scalarNode("service_started"))
			setMappingValue(value, -1, scalarNode("required"), &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
		}
		setMappingValue(result, -1, scalarNode(item.Value), value)
	}
	return result, true
}

func replaceComposeValue(path []string) bool {
	if len(path) == 4 && path[0] == "services" && path[2] == "healthcheck" && path[3] == "test" {
		return true
	}
	if len(path) != 3 || path[0] != "services" {
		return false
	}
	switch path[2] {
	case "command", "entrypoint":
		return true
	default:
		return false
	}
}

func appendUniqueComposeValue(sequence, value *yaml.Node, path []string) {
	identity, unique := composeSequenceIdentity(path, value)
	if !unique {
		sequence.Content = append(sequence.Content, value)
		return
	}
	for index, existing := range sequence.Content {
		existingIdentity, ok := composeSequenceIdentity(path, existing)
		if ok && existingIdentity == identity {
			sequence.Content[index] = value
			return
		}
	}
	sequence.Content = append(sequence.Content, value)
}

func composeSequenceIdentity(path []string, node *yaml.Node) (string, bool) {
	if len(path) != 3 || path[0] != "services" {
		return "", false
	}
	switch path[2] {
	case "environment", "labels", "annotations", "sysctls":
		if node.Kind == yaml.ScalarNode {
			key, _, _ := strings.Cut(node.Value, "=")
			return key, key != ""
		}
	case "volumes", "devices":
		return mountTarget(node)
	case "configs", "secrets":
		return resourceTarget(node)
	case "ports":
		return portIdentity(node)
	case "cap_add", "cap_drop", "dns", "dns_opt", "dns_search", "expose", "profiles", "tmpfs":
		if node.Kind == yaml.ScalarNode {
			return node.Value, true
		}
	case "env_file":
		if node.Kind == yaml.ScalarNode {
			return node.Value, true
		}
		if value, ok := mappingScalar(node, "path"); ok {
			return value, true
		}
	}
	return "", false
}

// UniqueResourceIdentity returns the Docker Compose merge identity for service
// resources whose sequence entries are merged independently.
func UniqueResourceIdentity(path []string, node *yaml.Node) (string, bool) {
	if !IsUniqueResourcePath(path) {
		return "", false
	}
	return composeSequenceIdentity(path, node)
}

// CanonicalUniqueResource normalizes equivalent short and long syntax before
// repository rebase compares an individual Compose resource.
func CanonicalUniqueResource(path []string, node *yaml.Node) (*yaml.Node, bool) {
	if !IsUniqueResourcePath(path) || node == nil {
		return nil, false
	}
	if node.Kind == yaml.MappingNode {
		if !knownUniqueResourceKeys(path, node) {
			return nil, false
		}
		var value any
		switch path[2] {
		case "volumes":
			var volume composetypes.ServiceVolumeConfig
			if err := node.Decode(&volume); err != nil {
				return nil, false
			}
			if volume.Type == "" {
				volume.Type = composetypes.VolumeTypeVolume
				if strings.HasPrefix(volume.Source, "/") || strings.HasPrefix(volume.Source, ".") || strings.HasPrefix(volume.Source, "~") {
					volume.Type = composetypes.VolumeTypeBind
				}
			}
			value = volume
		case "ports":
			var port composetypes.ServicePortConfig
			if err := node.Decode(&port); err != nil {
				return nil, false
			}
			if port.Protocol == "" {
				port.Protocol = "tcp"
			}
			if port.Mode == "" {
				port.Mode = "ingress"
			}
			value = port
		case "configs", "secrets":
			var reference composetypes.FileReferenceConfig
			if err := node.Decode(&reference); err != nil {
				return nil, false
			}
			if reference.Target == "" {
				reference.Target = reference.Source
			}
			value = reference
		}
		return marshalCanonicalNode(value)
	}
	if node.Kind != yaml.ScalarNode || strings.Contains(node.Value, "${") {
		return nil, false
	}
	var value any
	switch path[2] {
	case "volumes":
		volume, err := loader.ParseVolume(node.Value)
		if err != nil {
			return nil, false
		}
		value = volume
	case "ports":
		ports, err := composetypes.ParsePortConfig(node.Value)
		if err != nil || len(ports) != 1 {
			return nil, false
		}
		value = ports[0]
	case "configs", "secrets":
		value = composetypes.FileReferenceConfig{Source: node.Value, Target: node.Value}
	default:
		return nil, false
	}
	return marshalCanonicalNode(value)
}

func marshalCanonicalNode(value any) (*yaml.Node, bool) {
	content, err := yaml.Marshal(value)
	if err != nil {
		return nil, false
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil || len(document.Content) != 1 {
		return nil, false
	}
	return document.Content[0], true
}

func knownUniqueResourceKeys(path []string, node *yaml.Node) bool {
	allowed := map[string]struct{}{}
	switch path[2] {
	case "volumes":
		for _, key := range []string{"type", "source", "target", "read_only", "consistency", "bind", "volume", "tmpfs"} {
			allowed[key] = struct{}{}
		}
	case "ports":
		for _, key := range []string{"mode", "host_ip", "target", "published", "protocol"} {
			allowed[key] = struct{}{}
		}
	case "configs", "secrets":
		for _, key := range []string{"source", "target", "uid", "gid", "mode"} {
			allowed[key] = struct{}{}
		}
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if _, ok := allowed[node.Content[index].Value]; !ok {
			return false
		}
	}
	return true
}

func IsUniqueResourcePath(path []string) bool {
	if len(path) != 3 || path[0] != "services" {
		return false
	}
	switch path[2] {
	case "volumes", "ports", "configs", "secrets":
		return true
	default:
		return false
	}
}

func mountTarget(node *yaml.Node) (string, bool) {
	if value, ok := mappingScalar(node, "target"); ok {
		return value, true
	}
	if node.Kind != yaml.ScalarNode || strings.Contains(node.Value, "${") {
		return "", false
	}
	volume, err := loader.ParseVolume(node.Value)
	if err != nil {
		return "", false
	}
	return volume.Target, volume.Target != ""
}

func resourceTarget(node *yaml.Node) (string, bool) {
	if value, ok := mappingScalar(node, "target"); ok {
		return value, true
	}
	if value, ok := mappingScalar(node, "source"); ok {
		return value, true
	}
	if node.Kind == yaml.ScalarNode {
		return node.Value, node.Value != ""
	}
	return "", false
}

func portIdentity(node *yaml.Node) (string, bool) {
	if node.Kind == yaml.MappingNode {
		target, ok := mappingScalar(node, "target")
		if !ok {
			return "", false
		}
		published, _ := mappingScalar(node, "published")
		hostIP, _ := mappingScalar(node, "host_ip")
		protocol, _ := mappingScalar(node, "protocol")
		if protocol == "" {
			protocol = "tcp"
		}
		return strings.Join([]string{hostIP, published, target, protocol}, "\x00"), true
	}
	if node.Kind == yaml.ScalarNode && !strings.Contains(node.Value, "${") {
		ports, err := composetypes.ParsePortConfig(node.Value)
		if err != nil || len(ports) != 1 {
			return "", false
		}
		port := ports[0]
		return strings.Join([]string{port.HostIP, port.Published, fmt.Sprint(port.Target), port.Protocol}, "\x00"), true
	}
	return "", false
}

func materializeDocument(document *yaml.Node) (*yaml.Node, error) {
	if document == nil || len(document.Content) != 1 {
		return nil, fmt.Errorf("Compose YAML must contain one document")
	}
	root, err := materializeYAMLNode(document.Content[0], make(map[*yaml.Node]bool))
	if err != nil {
		return nil, err
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("Compose YAML must be a mapping")
	}
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}, nil
}

func materializeYAMLNode(node *yaml.Node, active map[*yaml.Node]bool) (*yaml.Node, error) {
	if node == nil {
		return nil, nil
	}
	if active[node] {
		return nil, fmt.Errorf("YAML alias cycle")
	}
	if node.Kind == yaml.AliasNode {
		active[node] = true
		resolved, err := materializeYAMLNode(node.Alias, active)
		delete(active, node)
		return resolved, err
	}
	copy := CloneNode(node)
	if node.Kind != yaml.MappingNode {
		for index, child := range node.Content {
			resolved, err := materializeYAMLNode(child, active)
			if err != nil {
				return nil, err
			}
			copy.Content[index] = resolved
		}
		return copy, nil
	}

	result := mappingNode()
	result.Tag = node.Tag
	result.Style = node.Style
	var mergeValues []*yaml.Node
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == "<<" {
			merged := node.Content[index+1]
			if merged.Kind == yaml.SequenceNode {
				mergeValues = append(mergeValues, merged.Content...)
			} else {
				mergeValues = append(mergeValues, merged)
			}
		}
	}
	for _, merged := range mergeValues {
		resolved, err := materializeYAMLNode(merged, active)
		if err != nil {
			return nil, err
		}
		if resolved == nil || resolved.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("YAML merge source must be a mapping")
		}
		for index := 0; index+1 < len(resolved.Content); index += 2 {
			key := resolved.Content[index].Value
			if mappingIndex(result, key) < 0 {
				setMappingValue(result, -1, resolved.Content[index], CloneNode(resolved.Content[index+1]))
			}
		}
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := node.Content[index]
		if key.Value == "<<" {
			continue
		}
		value, err := materializeYAMLNode(node.Content[index+1], active)
		if err != nil {
			return nil, err
		}
		setMappingValue(result, mappingIndex(result, key.Value), key, value)
	}
	return result, nil
}

func mappingScalar(node *yaml.Node, key string) (string, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", false
	}
	index := mappingIndex(node, key)
	if index < 0 || node.Content[index+1].Kind != yaml.ScalarNode {
		return "", false
	}
	return node.Content[index+1].Value, true
}

func mappingIndex(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return index
		}
	}
	return -1
}

func setMappingValue(mapping *yaml.Node, index int, key, value *yaml.Node) {
	if index >= 0 {
		mapping.Content[index+1] = value
		return
	}
	mapping.Content = append(mapping.Content, CloneNode(key), value)
}

func normalizeDirectiveTag(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Tag == resetTag || node.Tag == overrideTag {
		node.Style &^= yaml.TaggedStyle
		switch node.Kind {
		case yaml.MappingNode:
			node.Tag = "!!map"
		case yaml.SequenceNode:
			node.Tag = "!!seq"
		case yaml.ScalarNode:
			node.Tag = "!!str"
		}
	}
}
