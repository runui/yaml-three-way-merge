package compose

import (
	"errors"
	"strconv"

	"gopkg.in/yaml.v3"
)

func ParseMapping(content []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("Compose YAML must be a mapping")
	}
	return &document, nil
}

func PathValue(node *yaml.Node, path []string) (*yaml.Node, bool) {
	if node == nil || len(path) == 0 {
		return node, node != nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return PathValue(node.Content[0], path)
	}
	if node.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == path[0] {
			return PathValue(node.Content[index+1], path[1:])
		}
	}
	return nil, false
}

func SetPath(node *yaml.Node, path []string, value *yaml.Node) bool {
	if node == nil || len(path) == 0 {
		return false
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return SetPath(node.Content[0], path, value)
	}
	if node.Kind != yaml.MappingNode {
		return false
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value != path[0] {
			continue
		}
		if len(path) == 1 {
			node.Content[index+1] = value
			return true
		}
		child := node.Content[index+1]
		if child.Kind != yaml.MappingNode {
			child = mappingNode()
			node.Content[index+1] = child
		}
		return SetPath(child, path[1:], value)
	}
	child := value
	if len(path) > 1 {
		child = mappingNode()
	}
	node.Content = append(node.Content, scalarNode(path[0]), child)
	if len(path) == 1 {
		return true
	}
	return SetPath(child, path[1:], value)
}

func RemovePath(node *yaml.Node, path []string) bool {
	if node == nil || len(path) == 0 {
		return false
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return RemovePath(node.Content[0], path)
	}
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value != path[0] {
				continue
			}
			if len(path) == 1 {
				node.Content = append(node.Content[:index], node.Content[index+2:]...)
				return true
			}
			return RemovePath(node.Content[index+1], path[1:])
		}
	case yaml.SequenceNode:
		index, err := strconv.Atoi(path[0])
		if err != nil || index < 0 || index >= len(node.Content) {
			return false
		}
		if len(path) == 1 {
			node.Content = append(node.Content[:index], node.Content[index+1:]...)
			return true
		}
		return RemovePath(node.Content[index], path[1:])
	}
	return false
}

func CloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	cloned := *node
	cloned.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		cloned.Content[index] = CloneNode(child)
	}
	if node.Alias != nil {
		cloned.Alias = CloneNode(node.Alias)
	}
	return &cloned
}

func mappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
