package rebase

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	internalcompose "github.com/runui/yaml-three-way-merge/internal/compose"
	"gopkg.in/yaml.v3"
)

const containerLabelZimaAppID = "io.zimaos.app.id"

var (
	ErrInvalidBase = errors.New("invalid package base")
	ErrInvalid     = errors.New("invalid user override")
)

// Build derives a raw Compose override from an authoritative user-level document.
// The result is relative to this exact base. Sequence changes can require a
// list-level reset, so repository updates must rebase it before using a new base.
func Build(baseYAML, completeYAML []byte) ([]byte, error) {
	base, err := composeMappingDocument(baseYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: base YAML: %v", ErrInvalidBase, err)
	}
	complete, err := composeMappingDocument(completeYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: complete YAML: %v", ErrInvalid, err)
	}
	removeSystemOwnedComposeFields(base)
	removeSystemOwnedComposeFields(complete)

	reset, values := diffComposeNodes(nil, base, complete)
	if emptyMapping(reset) && emptyMapping(values) {
		return nil, nil
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if !emptyMapping(reset) {
		if err := encoder.Encode(documentNode(reset)); err != nil {
			return nil, fmt.Errorf("%w: encode resets: %v", ErrInvalid, err)
		}
	}
	if !emptyMapping(values) {
		if err := encoder.Encode(documentNode(values)); err != nil {
			return nil, fmt.Errorf("%w: encode values: %v", ErrInvalid, err)
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("%w: encode override: %v", ErrInvalid, err)
	}
	return output.Bytes(), nil
}

func ValidateComplete(content []byte) error {
	if _, err := composeMappingDocument(content); err != nil {
		return fmt.Errorf("%w: complete YAML: %v", ErrInvalid, err)
	}
	return nil
}

func projectValues(overrideYAML, completeYAML []byte) ([]byte, error) {
	if len(overrideYAML) == 0 {
		return nil, nil
	}
	complete, err := composeMappingDocumentPreservingSyntax(completeYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: complete YAML: %v", ErrInvalid, err)
	}
	var output bytes.Buffer
	decoder := yaml.NewDecoder(bytes.NewReader(overrideYAML))
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	for {
		var document yaml.Node
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("%w: decode generated override: %v", ErrInvalid, err)
		}
		projectOverrideNodeValues(&document, nil, complete)
		if err := encoder.Encode(&document); err != nil {
			return nil, fmt.Errorf("%w: encode generated override: %v", ErrInvalid, err)
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("%w: encode generated override: %v", ErrInvalid, err)
	}
	return output.Bytes(), nil
}

func projectOverrideNodeValues(node *yaml.Node, path []string, complete *yaml.Node) {
	if node == nil || node.Tag == "!reset" {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			projectOverrideNodeValues(child, path, complete)
		}
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			projectOverrideNodeValues(node.Content[index+1], appendPath(path, node.Content[index].Value), complete)
		}
	default:
		if raw, ok := internalcompose.PathValue(complete, path); ok {
			projected := internalcompose.CloneNode(raw)
			*node = *projected
		}
	}
}

func composeMappingDocumentPreservingSyntax(content []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("Compose YAML must be a mapping")
	}
	return &document, nil
}

func diffComposeNodes(path []string, base, complete *yaml.Node) (*yaml.Node, *yaml.Node) {
	reset := mappingNode()
	values := mappingNode()
	baseValues := mappingNodes(base)
	completeValues := mappingNodes(complete)
	for _, key := range sortedMappingKeys(baseValues) {
		baseValue := baseValues[key]
		completeValue, ok := completeValues[key]
		if !ok {
			appendMapping(reset, scalarNode(key), resetNode(baseValue))
			continue
		}
		childPath := appendPath(path, key)
		if internalcompose.SemanticNodesEqual(childPath, baseValue, completeValue) {
			continue
		}
		if baseValue.Kind == yaml.MappingNode && completeValue.Kind == yaml.MappingNode {
			childReset, childValues := diffComposeNodes(childPath, baseValue, completeValue)
			if !emptyMapping(childReset) {
				appendMapping(reset, scalarNode(key), childReset)
			}
			if !emptyMapping(childValues) {
				appendMapping(values, scalarNode(key), childValues)
			}
			continue
		}
		if completeValue.Tag == "!!null" || resetBeforeComposeMerge(baseValue, completeValue) {
			appendMapping(reset, scalarNode(key), resetNode(baseValue))
		}
		if completeValue.Tag != "!!null" {
			appendMapping(values, scalarNode(key), internalcompose.CloneNode(completeValue))
		}
	}
	for _, key := range sortedMappingKeys(completeValues) {
		completeValue := completeValues[key]
		if _, ok := baseValues[key]; ok {
			continue
		}
		appendMapping(values, scalarNode(key), internalcompose.CloneNode(completeValue))
	}
	return reset, values
}

func sortedMappingKeys(values map[string]*yaml.Node) []string {
	return slices.Sorted(maps.Keys(values))
}

func resetBeforeComposeMerge(base, complete *yaml.Node) bool {
	if base.Kind != complete.Kind || base.Kind == yaml.SequenceNode {
		return true
	}
	if complete.Kind != yaml.ScalarNode {
		return false
	}
	switch complete.Tag {
	case "!!bool":
		return complete.Value == "false"
	case "!!int", "!!float":
		return complete.Value == "0"
	case "!!str":
		return complete.Value == ""
	default:
		return false
	}
}

func composeMappingDocument(content []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("Compose YAML must be a mapping")
	}
	var normalized map[string]any
	if err := document.Content[0].Decode(&normalized); err != nil {
		return nil, err
	}
	normalizedContent, err := yaml.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var normalizedDocument yaml.Node
	if err := yaml.Unmarshal(normalizedContent, &normalizedDocument); err != nil {
		return nil, err
	}
	return normalizedDocument.Content[0], nil
}

func mappingNodes(node *yaml.Node) map[string]*yaml.Node {
	result := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		result[node.Content[index].Value] = node.Content[index+1]
	}
	return result
}

func removeMappingKey(node *yaml.Node, key string) {
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			node.Content = append(node.Content[:index], node.Content[index+2:]...)
			return
		}
	}
}

func removeSystemOwnedComposeFields(document *yaml.Node) {
	removeMappingKey(document, "name")
	removeMappingKey(document, "x-zima-app")
	services, ok := mappingValue(document, "services")
	if !ok || services.Kind != yaml.MappingNode {
		return
	}
	for index := 0; index+1 < len(services.Content); index += 2 {
		service := services.Content[index+1]
		if service.Kind != yaml.MappingNode {
			continue
		}
		removeMappingKey(service, "pull_policy")
		labels, ok := mappingValue(service, "labels")
		if !ok {
			continue
		}
		switch labels.Kind {
		case yaml.MappingNode:
			content := labels.Content[:0]
			for labelIndex := 0; labelIndex+1 < len(labels.Content); labelIndex += 2 {
				if systemOwnedLabel(labels.Content[labelIndex].Value) {
					continue
				}
				content = append(content, labels.Content[labelIndex], labels.Content[labelIndex+1])
			}
			labels.Content = content
		case yaml.SequenceNode:
			content := labels.Content[:0]
			for _, labelNode := range labels.Content {
				name := strings.SplitN(labelNode.Value, "=", 2)[0]
				if systemOwnedLabel(name) {
					continue
				}
				content = append(content, labelNode)
			}
			labels.Content = content
		}
		if len(labels.Content) == 0 {
			removeMappingKey(service, "labels")
		}
	}
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], true
		}
	}
	return nil, false
}

func appendPath(path []string, value string) []string {
	result := make([]string, len(path)+1)
	copy(result, path)
	result[len(path)] = value
	return result
}

func systemOwnedLabel(label string) bool {
	return strings.HasPrefix(label, "io.zimaos.") || strings.HasPrefix(label, "com.docker.compose.") || label == containerLabelZimaAppID
}

func resetNode(base *yaml.Node) *yaml.Node {
	switch base.Kind {
	case yaml.SequenceNode:
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!reset"}
	case yaml.MappingNode:
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!reset"}
	default:
		value := ""
		switch base.Tag {
		case "!!bool":
			value = "false"
		case "!!int", "!!float":
			value = "0"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!reset", Value: value}
	}
}

func mappingNode() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func appendMapping(mapping, key, value *yaml.Node) {
	mapping.Content = append(mapping.Content, key, value)
}

func emptyMapping(node *yaml.Node) bool { return node == nil || len(node.Content) == 0 }

func documentNode(content *yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{content}}
}
