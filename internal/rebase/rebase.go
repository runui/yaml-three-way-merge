package rebase

import (
	"errors"
	"fmt"
	"strings"

	internalcompose "github.com/runui/yaml-three-way-merge/internal/compose"
	"gopkg.in/yaml.v3"
)

type RepositoryRebaseOptions struct {
	ForceOverwriteImages bool
}

type nodeState struct {
	node    *yaml.Node
	present bool
}

// RebaseRepositoryUpdate merges the previous base, the user's current
// effective state, and the target base. Operation history is intentionally
// ignored: matching the previous base means the customization was canceled.
func RebaseRepositoryUpdate(previousBaseYAML, targetBaseYAML, userOverrideYAML []byte, options RepositoryRebaseOptions) ([]byte, error) {
	previousBase, err := parseRebaseDocument(previousBaseYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: previous base YAML: %v", ErrInvalidBase, err)
	}
	previousEffectiveYAML, err := internalcompose.MergeYAML(previousBaseYAML, userOverrideYAML)
	if err != nil {
		var overrideErr *internalcompose.OverrideError
		if errors.As(err, &overrideErr) {
			return nil, fmt.Errorf("%w: apply stored user override: %v", ErrInvalid, err)
		}
		return nil, fmt.Errorf("%w: previous base YAML: %v", ErrInvalidBase, err)
	}
	previousEffective, err := parseRebaseDocument(previousEffectiveYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: previous effective YAML: %v", ErrInvalid, err)
	}
	targetBase, err := parseRebaseDocument(targetBaseYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: target base YAML: %v", ErrInvalidBase, err)
	}

	rebased := mergeRebaseState(nil, presentNode(previousBase), presentNode(previousEffective), presentNode(targetBase))
	if !rebased.present {
		return nil, fmt.Errorf("%w: rebased Compose is empty", ErrInvalid)
	}
	if options.ForceOverwriteImages {
		restoreRepositoryImages(rebased.node, targetBase)
	}
	restoreRepositoryMetadata(rebased.node, targetBase)

	rebasedYAML, err := yaml.Marshal(rebased.node)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal rebased Compose: %v", ErrInvalid, err)
	}
	return Build(targetBaseYAML, rebasedYAML)
}

func parseRebaseDocument(content []byte) (*yaml.Node, error) {
	materialized, err := materializeAliases(content)
	if err != nil {
		return nil, err
	}
	document, err := internalcompose.ParseMapping(materialized)
	if err != nil {
		return nil, err
	}
	return document.Content[0], nil
}

func mergeRebaseState(path []string, previousBase, previousEffective, targetBase nodeState) nodeState {
	if rebaseStateEqual(path, previousEffective, previousBase) {
		return cloneState(targetBase)
	}
	if rebaseStateEqual(path, targetBase, previousBase) {
		return cloneState(previousEffective)
	}
	if rebaseStateEqual(path, previousEffective, targetBase) {
		return cloneState(previousEffective)
	}

	if isEnvironmentPath(path) && statesAreMappingsOrAbsent(previousBase, previousEffective, targetBase, true) {
		merged := mergeRebaseMapping(path, normalizeEnvironmentState(previousBase), normalizeEnvironmentState(previousEffective), normalizeEnvironmentState(targetBase))
		if merged.present && len(merged.node.Content) == 0 && !previousEffective.present {
			return nodeState{}
		}
		return merged
	}
	if internalcompose.IsUniqueResourcePath(path) && statesAreSequencesOrAbsent(previousBase, previousEffective, targetBase) {
		if merged, ok := mergeUniqueResourceSequence(path, previousBase, previousEffective, targetBase); ok {
			return merged
		}
	}
	if statesAreMappings(previousBase, previousEffective, targetBase) || statesAreAddedMappings(previousBase, previousEffective, targetBase) {
		return mergeRebaseMapping(path, previousBase, previousEffective, targetBase)
	}

	// Conflicts are resolved in favor of the user's current state. Mapping and
	// unique-resource recursion above still retains non-conflicting remote work.
	return cloneState(previousEffective)
}

func mergeRebaseMapping(path []string, previousBase, previousEffective, targetBase nodeState) nodeState {
	result := mappingNode()
	keys := orderedMappingKeys(targetBase, previousEffective, previousBase)
	for _, key := range keys {
		merged := mergeRebaseState(appendPath(path, key), mappingState(previousBase, key), mappingState(previousEffective, key), mappingState(targetBase, key))
		if merged.present {
			appendMapping(result, scalarNode(key), merged.node)
		}
	}
	if len(result.Content) == 0 && (!previousEffective.present || !targetBase.present) {
		return nodeState{}
	}
	return presentNode(result)
}

func mergeUniqueResourceSequence(path []string, previousBase, previousEffective, targetBase nodeState) (nodeState, bool) {
	baseItems, _, ok := indexedSequence(path, previousBase)
	if !ok {
		return nodeState{}, false
	}
	userItems, userOrder, ok := indexedSequence(path, previousEffective)
	if !ok {
		return nodeState{}, false
	}
	targetItems, targetOrder, ok := indexedSequence(path, targetBase)
	if !ok {
		return nodeState{}, false
	}
	// Compose merges these resources by identity rather than list position. This
	// prevents an old generated list reset from hiding new repository resources.
	result := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seen := make(map[string]struct{}, len(targetOrder)+len(userOrder))
	for _, identity := range append(targetOrder, userOrder...) {
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		merged := mergeRebaseState(appendPath(path, identity), baseItems[identity], userItems[identity], targetItems[identity])
		if merged.present {
			result.Content = append(result.Content, merged.node)
		}
	}
	if len(result.Content) == 0 && !previousEffective.present {
		return nodeState{}, true
	}
	return presentNode(result), true
}

func indexedSequence(path []string, state nodeState) (map[string]nodeState, []string, bool) {
	items := make(map[string]nodeState)
	if !state.present {
		return items, nil, true
	}
	if state.node.Kind != yaml.SequenceNode {
		return nil, nil, false
	}
	order := make([]string, 0, len(state.node.Content))
	for _, item := range state.node.Content {
		identity, ok := internalcompose.UniqueResourceIdentity(path, item)
		if !ok {
			// Do not guess identities for interpolation or unsupported syntax;
			// treating the list atomically is safer than duplicating resources.
			return nil, nil, false
		}
		if _, duplicate := items[identity]; duplicate {
			return nil, nil, false
		}
		items[identity] = presentNode(item)
		order = append(order, identity)
	}
	return items, order, true
}

func normalizeEnvironmentState(state nodeState) nodeState {
	if !state.present {
		return presentNode(mappingNode())
	}
	if state.node.Kind == yaml.MappingNode {
		return cloneState(state)
	}
	result := mappingNode()
	for _, item := range state.node.Content {
		key, value, found := strings.Cut(item.Value, "=")
		if !found {
			valueNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
			appendMapping(result, scalarNode(key), valueNode)
			continue
		}
		appendMapping(result, scalarNode(key), scalarNode(value))
	}
	return presentNode(result)
}

func restoreRepositoryImages(rebased, targetBase *yaml.Node) {
	targetServices, found := internalcompose.PathValue(targetBase, []string{"services"})
	if !found || targetServices.Kind != yaml.MappingNode {
		return
	}
	for index := 0; index+1 < len(targetServices.Content); index += 2 {
		service := targetServices.Content[index].Value
		image, found := internalcompose.PathValue(targetServices.Content[index+1], []string{"image"})
		if !found {
			continue
		}
		if _, survives := internalcompose.PathValue(rebased, []string{"services", service}); survives {
			internalcompose.SetPath(rebased, []string{"services", service, "image"}, internalcompose.CloneNode(image))
		}
	}
}

func restoreRepositoryMetadata(rebased, targetBase *yaml.Node) {
	versionPath := []string{"x-casaos", "version"}
	if version, found := internalcompose.PathValue(targetBase, versionPath); found {
		internalcompose.SetPath(rebased, versionPath, internalcompose.CloneNode(version))
	} else {
		internalcompose.RemovePath(rebased, versionPath)
	}
	repoPath := []string{"x-casaos", "repo_id"}
	if repoID, found := internalcompose.PathValue(targetBase, repoPath); found {
		internalcompose.SetPath(rebased, repoPath, internalcompose.CloneNode(repoID))
	}
}

func rebaseStateEqual(path []string, left, right nodeState) bool {
	if left.present != right.present {
		return false
	}
	if !left.present {
		return true
	}
	if isEnvironmentPath(path) {
		left = normalizeEnvironmentState(left)
		right = normalizeEnvironmentState(right)
	}
	if internalcompose.IsUniqueResourcePath(path) {
		return uniqueResourceSequencesEqual(path, left, right)
	}
	if len(path) == 4 && internalcompose.IsUniqueResourcePath(path[:3]) {
		leftNode, leftOK := internalcompose.CanonicalUniqueResource(path[:3], left.node)
		rightNode, rightOK := internalcompose.CanonicalUniqueResource(path[:3], right.node)
		if leftOK && rightOK {
			return internalcompose.SemanticNodesEqual(path, leftNode, rightNode)
		}
	}
	return internalcompose.SemanticNodesEqual(path, left.node, right.node)
}

func uniqueResourceSequencesEqual(path []string, left, right nodeState) bool {
	leftItems, _, leftOK := indexedSequence(path, left)
	rightItems, _, rightOK := indexedSequence(path, right)
	if !leftOK || !rightOK || len(leftItems) != len(rightItems) {
		return false
	}
	for identity, leftItem := range leftItems {
		rightItem, exists := rightItems[identity]
		if !exists || !rebaseStateEqual(appendPath(path, identity), leftItem, rightItem) {
			return false
		}
	}
	return true
}

func mappingState(state nodeState, key string) nodeState {
	if !state.present || state.node.Kind != yaml.MappingNode {
		return nodeState{}
	}
	for index := 0; index+1 < len(state.node.Content); index += 2 {
		if state.node.Content[index].Value == key {
			return presentNode(state.node.Content[index+1])
		}
	}
	return nodeState{}
}

func orderedMappingKeys(states ...nodeState) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, state := range states {
		if !state.present || state.node.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(state.node.Content); index += 2 {
			key := state.node.Content[index].Value
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

func statesAreMappings(states ...nodeState) bool {
	for _, state := range states {
		if !state.present || state.node.Kind != yaml.MappingNode {
			return false
		}
	}
	return true
}

func statesAreAddedMappings(previousBase, previousEffective, targetBase nodeState) bool {
	return !previousBase.present && previousEffective.present && targetBase.present && previousEffective.node.Kind == yaml.MappingNode && targetBase.node.Kind == yaml.MappingNode
}

func statesAreMappingsOrAbsent(previousBase, previousEffective, targetBase nodeState, allowSequence bool) bool {
	for _, state := range []nodeState{previousBase, previousEffective, targetBase} {
		if !state.present || state.node.Kind == yaml.MappingNode || allowSequence && state.node.Kind == yaml.SequenceNode {
			continue
		}
		return false
	}
	return true
}

func statesAreSequencesOrAbsent(states ...nodeState) bool {
	for _, state := range states {
		if state.present && state.node.Kind != yaml.SequenceNode {
			return false
		}
	}
	return true
}

func isEnvironmentPath(path []string) bool {
	return len(path) == 3 && path[0] == "services" && path[2] == "environment"
}

func presentNode(node *yaml.Node) nodeState {
	return nodeState{node: internalcompose.CloneNode(node), present: true}
}

func cloneState(state nodeState) nodeState {
	if !state.present {
		return nodeState{}
	}
	return presentNode(state.node)
}
