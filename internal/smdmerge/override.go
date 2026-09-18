package smdmerge

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type overrideDocument struct {
	resets [][]string
	yaml   []byte
}

func parseOverrideDocuments(content []byte) ([]overrideDocument, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var result []overrideDocument
	for {
		var document yaml.Node
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse override: %w", err)
		}
		if len(document.Content) == 0 {
			continue
		}
		root := document.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("override document must be a mapping")
		}
		var resets [][]string
		if err := stripResetNodes(root, nil, &resets); err != nil {
			return nil, err
		}
		var encoded []byte
		if len(root.Content) > 0 {
			var output bytes.Buffer
			encoder := yaml.NewEncoder(&output)
			encoder.SetIndent(2)
			if err := encoder.Encode(&document); err != nil {
				return nil, fmt.Errorf("encode override writes: %w", err)
			}
			if err := encoder.Close(); err != nil {
				return nil, err
			}
			encoded = output.Bytes()
		}
		result = append(result, overrideDocument{resets: resets, yaml: encoded})
	}
	return result, nil
}

func stripResetNodes(node *yaml.Node, path []string, resets *[][]string) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	kept := node.Content[:0]
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		next := appendPath(path, key.Value)
		if value.Tag == "!reset" {
			*resets = append(*resets, append([]string(nil), next...))
			continue
		}
		if err := stripResetNodes(value, next, resets); err != nil {
			return err
		}
		kept = append(kept, key, value)
	}
	node.Content = kept
	return nil
}

func appendPath(path []string, value string) []string {
	result := make([]string, len(path)+1)
	copy(result, path)
	result[len(path)] = value
	return result
}
