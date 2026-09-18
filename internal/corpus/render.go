package corpus

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

func renderCase(field FieldSpec, id, scenario, description string, base, user, remote, expected []any) (Case, error) {
	baseYAML, err := renderComplete(field, base)
	if err != nil {
		return Case{}, err
	}
	userEffective, err := renderComplete(field, user)
	if err != nil {
		return Case{}, err
	}
	remoteYAML, err := renderComplete(field, remote)
	if err != nil {
		return Case{}, err
	}
	expectedYAML, err := renderComplete(field, expected)
	if err != nil {
		return Case{}, err
	}
	userOverride, err := renderOverride(baseYAML, userEffective)
	if err != nil {
		return Case{}, err
	}
	return Case{
		Metadata: Metadata{ID: id, Field: joinPath(field.Path), Class: field.Class, Scenario: scenario, Description: description},
		BaseOld:  baseYAML, User: userOverride, BaseNew: remoteYAML, Expected: expectedYAML,
		UserEffective: userEffective,
	}, nil
}

func renderComplete(field FieldSpec, values []any) ([]byte, error) {
	root := map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest"}}}
	if len(values) > 0 {
		if field.Wrap != nil {
			root = field.Wrap(values)
		} else {
			setPath(root, field.Path, values)
		}
	}
	return yaml.Marshal(root)
}

func renderOverride(baseYAML, userYAML []byte) ([]byte, error) {
	if bytes.Equal(baseYAML, userYAML) {
		return nil, nil
	}
	var baseDocument, userDocument yaml.Node
	if err := yaml.Unmarshal(baseYAML, &baseDocument); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(userYAML, &userDocument); err != nil {
		return nil, err
	}
	reset := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	baseRoot := baseDocument.Content[0]
	for index := 0; index+1 < len(baseRoot.Content); index += 2 {
		key := baseRoot.Content[index]
		value := baseRoot.Content[index+1]
		resetValue := &yaml.Node{Kind: value.Kind, Tag: "!reset"}
		reset.Content = append(reset.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.Value}, resetValue)
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{reset}}); err != nil {
		return nil, err
	}
	if err := encoder.Encode(&userDocument); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func mappingPath(path []string, value *yaml.Node) *yaml.Node {
	for index := len(path) - 1; index >= 0; index-- {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: path[index]}, value,
		}}
	}
	return value
}

func valueNode(value any) (*yaml.Node, error) {
	content, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("marshal value produced %d documents", len(document.Content))
	}
	return document.Content[0], nil
}

func setPath(root map[string]any, path []string, value any) {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func joinPath(path []string) string {
	var output bytes.Buffer
	for index, item := range path {
		if index > 0 {
			output.WriteByte('.')
		}
		output.WriteString(item)
	}
	return output.String()
}
