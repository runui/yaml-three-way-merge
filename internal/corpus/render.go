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
	userOverride, err := renderOverride(field, baseYAML, userEffective)
	if err != nil {
		return Case{}, err
	}
	return Case{
		Metadata: Metadata{ID: id, Field: joinPath(field.Path), Class: field.Class, Scenario: scenario, Description: description, PairSemantics: field.PairSemantics},
		BaseOld:  baseYAML, User: userOverride, BaseNew: remoteYAML, Expected: expectedYAML, UserEffective: userEffective,
	}, nil
}

func renderComplete(field FieldSpec, values []any) ([]byte, error) {
	root := map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest"}}}
	if field.Wrap != nil {
		wrapped := field.Wrap(values, len(values) > 0)
		mergeMaps(root, wrapped)
	} else if len(values) > 0 {
		setPath(root, field.Path, values)
	}
	return yaml.Marshal(root)
}

func renderOverride(field FieldSpec, baseYAML, userYAML []byte) ([]byte, error) {
	base, err := decodeMap(baseYAML)
	if err != nil {
		return nil, err
	}
	user, err := decodeMap(userYAML)
	if err != nil {
		return nil, err
	}
	boundary := field.ResetBoundary
	if len(boundary) == 0 {
		boundary = field.Path
	}
	baseValue, basePresent := getPath(base, boundary)
	userValue, userPresent := getPath(user, boundary)
	if semanticEqual(baseValue, basePresent, userValue, userPresent) {
		return nil, nil
	}
	resetBoundary := boundary
	if !userPresent {
		resetBoundary = cleanupBoundary(base, user, boundary)
		baseValue, basePresent = getPath(base, resetBoundary)
	}

	reset := map[string]any{}
	if basePresent {
		setPath(reset, resetBoundary, resetNode(baseValue))
	}
	var documents [][]byte
	if basePresent {
		resetYAML, err := yaml.Marshal(reset)
		if err != nil {
			return nil, err
		}
		documents = append(documents, resetYAML)
	}
	if userPresent {
		patch := map[string]any{}
		setPath(patch, resetBoundary, userValue)
		valueYAML, err := yaml.Marshal(patch)
		if err != nil {
			return nil, err
		}
		documents = append(documents, valueYAML)
	}
	if len(documents) == 0 {
		return nil, nil
	}
	return bytes.Join(documents, []byte("---\n")), nil
}

func decodeMap(content []byte) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func resetNode(value any) any {
	kind := yaml.SequenceNode
	switch value.(type) {
	case map[string]any:
		kind = yaml.MappingNode
	}
	return &yaml.Node{Kind: kind, Tag: "!reset", Content: nil}
}

func cleanupBoundary(base, user map[string]any, boundary []string) []string {
	current := append([]string(nil), boundary...)
	for len(current) > 1 {
		parent := current[:len(current)-1]
		baseParent, baseOK := getPath(base, parent)
		_, userOK := getPath(user, parent)
		mapping, mappingOK := baseParent.(map[string]any)
		if !baseOK || userOK || !mappingOK || len(mapping) != 1 {
			break
		}
		current = parent
	}
	return current
}

func semanticEqual(a any, aPresent bool, b any, bPresent bool) bool {
	if aPresent != bPresent {
		return false
	}
	if !aPresent {
		return true
	}
	aYAML, _ := yaml.Marshal(a)
	bYAML, _ := yaml.Marshal(b)
	return bytes.Equal(aYAML, bYAML)
}

func getPath(root map[string]any, path []string) (any, bool) {
	var current any = root
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapping[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func mergeMaps(dst, src map[string]any) {
	for key, value := range src {
		srcMap, srcOK := value.(map[string]any)
		dstMap, dstOK := dst[key].(map[string]any)
		if srcOK && dstOK {
			mergeMaps(dstMap, srcMap)
			continue
		}
		dst[key] = value
	}
}

func mappingPath(path []string, value any) map[string]any {
	root := map[string]any{}
	setPath(root, path, value)
	return root
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
