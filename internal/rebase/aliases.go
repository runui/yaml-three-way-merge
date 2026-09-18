package rebase

import (
	"bytes"
	"io"

	internalcompose "github.com/runui/yaml-three-way-merge/internal/compose"
	"gopkg.in/yaml.v3"
)

func materializeAliases(content []byte) ([]byte, error) {
	var output bytes.Buffer
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	for {
		var document yaml.Node
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(document.Content) == 0 {
			continue
		}
		expandAliases(&document)
		if err := encoder.Encode(&document); err != nil {
			return nil, err
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func expandAliases(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.AliasNode {
		*node = *internalcompose.CloneNode(node.Alias)
	}
	for _, child := range node.Content {
		expandAliases(child)
	}
}
