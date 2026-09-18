package compose

import "gopkg.in/yaml.v3"

// SemanticYAMLEqual compares Compose documents while accounting for the
// unique-resource sequences whose order is not significant during a merge.
func SemanticYAMLEqual(left, right []byte) (bool, error) {
	leftNode, err := ParseMapping(left)
	if err != nil {
		return false, err
	}
	rightNode, err := ParseMapping(right)
	if err != nil {
		return false, err
	}
	return SemanticNodesEqual(nil, leftNode, rightNode), nil
}

func SemanticNodesEqual(path []string, left, right *yaml.Node) bool {
	left = semanticNode(left)
	right = semanticNode(right)
	if left == nil || right == nil {
		return left == right
	}
	if left.Kind != right.Kind || left.Tag != right.Tag {
		return false
	}
	switch left.Kind {
	case yaml.MappingNode:
		return semanticMappingsEqual(path, left, right)
	case yaml.SequenceNode:
		if len(left.Content) != len(right.Content) {
			return false
		}
		if composeUniqueResourcePath(path) {
			return semanticUnorderedSequencesEqual(path, left, right)
		}
		for index := range left.Content {
			if !SemanticNodesEqual(path, left.Content[index], right.Content[index]) {
				return false
			}
		}
		return true
	case yaml.ScalarNode:
		return left.Value == right.Value
	default:
		return left.Value == right.Value && len(left.Content) == len(right.Content)
	}
}

func semanticNode(node *yaml.Node) *yaml.Node {
	for node != nil {
		switch {
		case node.Kind == yaml.DocumentNode && len(node.Content) == 1:
			node = node.Content[0]
		case node.Kind == yaml.AliasNode && node.Alias != nil:
			node = node.Alias
		default:
			return node
		}
	}
	return nil
}

func semanticMappingsEqual(path []string, left, right *yaml.Node) bool {
	if len(left.Content) != len(right.Content) {
		return false
	}
	rightValues := make(map[string]*yaml.Node, len(right.Content)/2)
	for index := 0; index+1 < len(right.Content); index += 2 {
		rightValues[right.Content[index].Value] = right.Content[index+1]
	}
	if len(rightValues) != len(right.Content)/2 {
		return false
	}
	seen := make(map[string]struct{}, len(left.Content)/2)
	for index := 0; index+1 < len(left.Content); index += 2 {
		key := left.Content[index].Value
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
		rightValue, exists := rightValues[key]
		if !exists || !SemanticNodesEqual(semanticPath(path, key), left.Content[index+1], rightValue) {
			return false
		}
	}
	return true
}

func semanticUnorderedSequencesEqual(path []string, left, right *yaml.Node) bool {
	matched := make([]bool, len(right.Content))
	for _, leftValue := range left.Content {
		found := false
		for index, rightValue := range right.Content {
			if matched[index] || !SemanticNodesEqual(path, leftValue, rightValue) {
				continue
			}
			matched[index] = true
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

func composeUniqueResourcePath(path []string) bool {
	if len(path) != 3 || path[0] != "services" {
		return false
	}
	switch path[2] {
	case "volumes", "ports", "secrets", "configs":
		return true
	default:
		return false
	}
}

func semanticPath(path []string, value string) []string {
	result := make([]string, len(path)+1)
	copy(result, path)
	result[len(path)] = value
	return result
}
