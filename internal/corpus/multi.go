package corpus

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file adds two scenario dimensions that the per-field state matrix
// cannot express:
//
//   - multi-resource: several services, networks, volumes, configs or secrets
//     are present and only one of them is changed by a side. This checks that
//     merge scope is the changed resource, not the whole document.
//   - multi-attribute: an array item carries several fields and only one or
//     several of those fields change. This checks attribute-level scope inside
//     a single logical item.
//
// Both families are curated rather than exhaustive; their counts are reported
// through ExplicitCaseCount so the manifest completeness test can account for
// them.

// --- shared helpers --------------------------------------------------------

func completeRoot(field FieldSpec, values []any) (map[string]any, error) {
	content, err := renderComplete(field, values)
	if err != nil {
		return nil, err
	}
	return decodeMap(content)
}

func marshalRoot(root map[string]any) ([]byte, error) {
	if len(root) == 0 {
		return nil, nil
	}
	return yaml.Marshal(root)
}

// renderMultiOverride builds a user overlay that resets and rewrites each
// changed boundary independently, so a document may express several resource
// or attribute changes without resetting an unrelated parent.
func renderMultiOverride(oldRoot, userRoot map[string]any, boundaries [][]string) ([]byte, error) {
	oldYAML, err := marshalRoot(oldRoot)
	if err != nil {
		return nil, err
	}
	userYAML, err := marshalRoot(userRoot)
	if err != nil {
		return nil, err
	}
	var documents [][]byte
	for _, boundary := range boundaries {
		content, err := renderOverride(FieldSpec{ResetBoundary: boundary}, oldYAML, userYAML)
		if err != nil {
			return nil, err
		}
		if len(content) > 0 {
			documents = append(documents, content)
		}
	}
	if len(documents) == 0 {
		return nil, nil
	}
	return bytes.Join(documents, []byte("---\n")), nil
}

func renderExplicitCase(id, fieldPath, scenario, description string, boundaries [][]string, oldRoot, userRoot, newRoot, expectedRoot map[string]any) (Case, error) {
	oldYAML, err := marshalRoot(oldRoot)
	if err != nil {
		return Case{}, err
	}
	newYAML, err := marshalRoot(newRoot)
	if err != nil {
		return Case{}, err
	}
	expectedYAML, err := marshalRoot(expectedRoot)
	if err != nil {
		return Case{}, err
	}
	userEffective, err := marshalRoot(userRoot)
	if err != nil {
		return Case{}, err
	}
	override, err := renderMultiOverride(oldRoot, userRoot, boundaries)
	if err != nil {
		return Case{}, err
	}
	return Case{
		Metadata: Metadata{
			ID: id, Field: fieldPath, Class: "multi",
			Scenario: scenario, Description: description, PairSemantics: PairByLogicalItem,
		},
		Dir: id, BaseOld: oldYAML, User: override, BaseNew: newYAML,
		Expected: expectedYAML, UserEffective: userEffective,
	}, nil
}

func mergeRoots(roots ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, root := range roots {
		mergeMaps(merged, root)
	}
	return merged
}

// --- multi-resource scenarios ----------------------------------------------

// resourceOwner returns the grouping key that owns a field path and the name of
// the sibling resource used to prove the merge does not leak across resources.
// Only paths with real mapping segments qualify; display paths containing "[]"
// notation are skipped.
func resourceOwner(path []string) (owner []string, sibling string, ok bool) {
	if len(path) < 3 {
		return nil, "", false
	}
	for _, element := range path {
		if strings.Contains(element, "[]") {
			return nil, "", false
		}
	}
	switch path[0] {
	case "services":
		if path[1] != "app" {
			return nil, "", false
		}
		return []string{"services", "app"}, "sidecar", true
	case "networks", "volumes", "configs", "secrets":
		if path[1] != "default" {
			return nil, "", false
		}
		return []string{path[0], path[1]}, "other", true
	default:
		return nil, "", false
	}
}

func renameOwner(root map[string]any, owner []string, sibling string) map[string]any {
	group, ok := root[owner[0]].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	value, ok := group[owner[1]]
	if !ok {
		return map[string]any{}
	}
	delete(group, owner[1])
	group[sibling] = value
	return root
}

func siblingBoundary(boundary []string, sibling string) []string {
	renamed := append([]string(nil), boundary...)
	renamed[1] = sibling
	return renamed
}

func multiResourceCases() ([]Case, error) {
	var cases []Case
	for _, field := range ArrayFields() {
		owner, sibling, ok := resourceOwner(field.ResetBoundary)
		if !ok {
			continue
		}
		appBase, sideBase := fieldSlot(field, 0)
		appUser, sideUser := fieldSlot(field, 1)
		appRemote, sideRemote := fieldSlot(field, 2)
		renderApp := func(value any) map[string]any {
			root, err := completeRoot(field, slotValues(value))
			if err != nil {
				return map[string]any{}
			}
			return root
		}
		renderSide := func(value any) map[string]any {
			root, err := completeRoot(field, slotValues(value))
			if err != nil {
				return map[string]any{}
			}
			return renameOwner(root, owner, sibling)
		}
		boundaries := [][]string{field.ResetBoundary, siblingBoundary(field.ResetBoundary, sibling)}
		scenarios := []struct {
			name                  string
			description           string
			oldApp, oldSide       any
			userApp, userSide     any
			newApp, newSide       any
			expectApp, expectSide any
		}{
			{
				"user-app-remote-sibling", "user changes only app, newbase changes only the sibling resource",
				appBase, sideBase, appUser, sideBase, appBase, sideRemote, appUser, sideRemote,
			},
			{
				"user-delete-app-remote-modify-sibling", "user removes the field on app, newbase changes the sibling resource",
				appBase, sideBase, nil, sideBase, appBase, sideRemote, nil, sideRemote,
			},
			{
				"user-modify-app-remote-delete-sibling", "user changes app, newbase removes the field on the sibling resource",
				appBase, sideBase, appUser, sideBase, appBase, nil, appUser, nil,
			},
			{
				"both-modify-user-wins", "both sides change app and the sibling resource; user intent wins",
				appBase, sideBase, appUser, sideUser, appRemote, sideRemote, appUser, sideUser,
			},
		}
		for _, scenario := range scenarios {
			oldRoot := mergeRoots(renderApp(scenario.oldApp), renderSide(scenario.oldSide))
			userRoot := mergeRoots(renderApp(scenario.userApp), renderSide(scenario.userSide))
			newRoot := mergeRoots(renderApp(scenario.newApp), renderSide(scenario.newSide))
			expectedRoot := mergeRoots(renderApp(scenario.expectApp), renderSide(scenario.expectSide))
			id := filepath.Join("multi", field.Class.String(), field.ID, scenario.name)
			generated, err := renderExplicitCase(id, joinPath(field.ResetBoundary), scenario.name, scenario.description, boundaries, oldRoot, userRoot, newRoot, expectedRoot)
			if err != nil {
				return nil, fmt.Errorf("generate %s: %w", id, err)
			}
			cases = append(cases, generated)
		}
	}
	return cases, nil
}

// fieldSlot returns the value of a logical slot for the primary and secondary
// logical items. Atomic fields have no second item, so both resources carry the
// same value shape.
func fieldSlot(field FieldSpec, slot int) (primary, secondary any) {
	primary = field.Values[slot]
	if field.CrossProduct {
		secondary = field.SecondValues[slot]
	} else {
		secondary = field.Values[slot]
	}
	return primary, secondary
}

func slotValues(value any) []any {
	if value == nil {
		return nil
	}
	return []any{value}
}

func (c MergeClass) String() string { return string(c) }

// --- multi-attribute scenarios ---------------------------------------------

type itemAttr struct {
	name   string
	base   any
	user   any
	remote any
}

type multiAttrSpec struct {
	id        string
	path      []string
	identity  map[string]any
	attrs     []itemAttr
	wholeList bool
	wrap      func(item map[string]any) map[string]any
	scaffold  map[string]any
	// expect overrides the default scalar-attribute expectation for items
	// whose attributes are themselves logical-item sequences. Such attributes
	// follow their own element semantics instead of whole-value user-wins.
	expect func(userItem, remoteItem map[string]any) map[string]any
}

func multiAttributeCases() ([]Case, error) {
	var cases []Case
	for _, spec := range multiAttrSpecs() {
		// A disjoint scenario only proves something when each side changes a
		// different attribute. Two-valued attributes whose remote value equals
		// the base value cannot carry the remote change, so the roles swap.
		userAttr, remoteAttr := spec.attrs[0], spec.attrs[1]
		if fmt.Sprint(remoteAttr.remote) == fmt.Sprint(remoteAttr.base) {
			userAttr, remoteAttr = remoteAttr, userAttr
		}
		base := spec.item(nil)
		userDisjoint := spec.item(map[string]any{userAttr.name: userAttr.user})
		remoteDisjoint := spec.item(map[string]any{remoteAttr.name: remoteAttr.remote})
		expectedDisjoint := spec.expected(userDisjoint, remoteDisjoint)
		userOverlap := spec.item(map[string]any{
			spec.attrs[0].name: spec.attrs[0].user,
			spec.attrs[1].name: spec.attrs[1].user,
		})
		remoteOverlap := spec.item(map[string]any{
			spec.attrs[0].name: spec.attrs[0].remote,
			spec.attrs[1].name: spec.attrs[1].remote,
		})
		expectedOverlap := spec.expected(userOverlap, remoteOverlap)

		type scenario struct {
			name        string
			description string
			old, user   map[string]any
			new         map[string]any
			expected    map[string]any
		}
		scenarios := []scenario{
			{
				"disjoint-attributes", "user and newbase change different attributes of the same array item",
				spec.root(base), spec.root(userDisjoint), spec.root(remoteDisjoint), spec.root(expectedDisjoint),
			},
			{
				"overlapping-attributes-user-wins", "both sides change the same attributes; user intent wins",
				spec.root(base), spec.root(userOverlap), spec.root(remoteOverlap), spec.root(expectedOverlap),
			},
			{
				"user-modify-remote-delete-item", "user changes one attribute while newbase removes the whole item",
				spec.root(base), spec.root(userDisjoint), spec.removed(), spec.root(userDisjoint),
			},
			{
				"user-delete-remote-modify-item", "user removes the whole item while newbase changes one attribute",
				spec.root(base), spec.removed(), spec.root(remoteDisjoint), spec.removed(),
			},
		}
		for _, scenario := range scenarios {
			id := filepath.Join("multi-attribute", spec.id, scenario.name)
			generated, err := renderExplicitCase(id, joinPath(spec.path), scenario.name, scenario.description, [][]string{spec.path}, scenario.old, scenario.user, scenario.new, scenario.expected)
			if err != nil {
				return nil, fmt.Errorf("generate %s: %w", id, err)
			}
			cases = append(cases, generated)
		}
	}
	return cases, nil
}

func (s multiAttrSpec) item(overrides map[string]any) map[string]any {
	item := make(map[string]any, len(s.identity)+len(s.attrs))
	for key, value := range s.identity {
		item[key] = value
	}
	for _, attr := range s.attrs {
		item[attr.name] = attr.base
	}
	for key, value := range overrides {
		item[key] = value
	}
	return item
}

func (s multiAttrSpec) root(item map[string]any) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return s.wrap(item)
}

func (s multiAttrSpec) removed() map[string]any {
	return s.scaffold
}

// expected applies the declared user-intent rule for the item: an attribute the
// user changed keeps the user value, an untouched attribute follows newbase.
// Whole-list fields have no per-attribute identity, so any user change replaces
// the list.
func (s multiAttrSpec) expected(userItem, remoteItem map[string]any) map[string]any {
	if s.wholeList {
		return userItem
	}
	if userItem == nil {
		return nil
	}
	result := make(map[string]any, len(userItem))
	for key, value := range userItem {
		result[key] = value
	}
	for key, value := range remoteItem {
		if s.userChanged(userItem, key) {
			continue
		}
		result[key] = value
	}
	return result
}

func (s multiAttrSpec) userChanged(userItem map[string]any, key string) bool {
	if userItem == nil {
		return false
	}
	userValue, exists := userItem[key]
	if !exists {
		return false
	}
	baseValue, baseExists := s.item(nil)[key]
	return !baseExists || fmt.Sprint(userValue) != fmt.Sprint(baseValue)
}

func multiAttrSpecs() []multiAttrSpec {
	identity := func(pairs ...any) map[string]any {
		result := make(map[string]any, len(pairs)/2)
		for index := 0; index+1 < len(pairs); index += 2 {
			result[pairs[index].(string)] = pairs[index+1]
		}
		return result
	}
	serviceScaffold := map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest"}}}
	return []multiAttrSpec{
		{
			id: "service.ports", path: []string{"services", "app", "ports"},
			identity: identity("target", 80, "protocol", "tcp"),
			attrs: []itemAttr{
				{name: "published", base: "8080", user: "8181", remote: "8282"},
				{name: "mode", base: "host", user: "ingress", remote: "host"},
			},
			wrap:     func(item map[string]any) map[string]any { return serviceMapping("ports", []any{item}) },
			scaffold: serviceScaffold,
		},
		{
			id: "service.volumes", path: []string{"services", "app", "volumes"},
			identity: identity("target", "/container", "type", "bind"),
			attrs: []itemAttr{
				{name: "source", base: "/base", user: "/user", remote: "/remote"},
				{name: "read_only", base: false, user: true, remote: false},
			},
			wrap:     func(item map[string]any) map[string]any { return serviceMapping("volumes", []any{item}) },
			scaffold: serviceScaffold,
		},
		{
			id: "service.configs", path: []string{"services", "app", "configs"},
			identity: identity("target", "/config", "source", "base_config"),
			attrs: []itemAttr{
				{name: "uid", base: "1000", user: "1001", remote: "1002"},
				{name: "mode", base: 0444, user: 0440, remote: 0400},
			},
			wrap:     func(item map[string]any) map[string]any { return serviceMapping("configs", []any{item}) },
			scaffold: serviceScaffold,
		},
		{
			id: "service.secrets", path: []string{"services", "app", "secrets"},
			identity: identity("target", "/run/secret", "source", "base_secret"),
			attrs: []itemAttr{
				{name: "uid", base: "1000", user: "1001", remote: "1002"},
				{name: "gid", base: "2000", user: "2001", remote: "2002"},
			},
			wrap:     func(item map[string]any) map[string]any { return serviceMapping("secrets", []any{item}) },
			scaffold: serviceScaffold,
		},
		{
			// service devices are authored as strings in the pinned Compose
			// version, so there are no independently addressable item
			// attributes to model here; the whole element is the logical item.
			id: "deploy.resources.reservations.devices", path: []string{"services", "app", "deploy", "resources", "reservations", "devices"},
			identity: identity("driver", "nvidia"),
			attrs: []itemAttr{
				// The trailing token after the last dash is the logical-item
				// identity, so both sides change the same element.
				{name: "capabilities", base: []any{"gpu-a"}, user: []any{"tpu-a"}, remote: []any{"npu-a"}},
				{name: "device_ids", base: []any{"idbase-a"}, user: []any{"iduser-a"}, remote: []any{"idremote-a"}},
			},
			wrap: func(item map[string]any) map[string]any {
				return serviceMapping("deploy", map[string]any{"resources": map[string]any{"reservations": map[string]any{"devices": []any{item}}}})
			},
			scaffold: serviceScaffold,
		},
		{
			id: "network.ipam.config", path: []string{"networks", "default", "ipam", "config"},
			identity: identity("subnet", "10.0.0.0/24"),
			attrs: []itemAttr{
				{name: "ip_range", base: "10.0.0.0/25", user: "10.0.0.0/26", remote: "10.0.0.0/27"},
				{name: "gateway", base: "10.0.0.1", user: "10.0.0.2", remote: "10.0.0.3"},
			},
			wrap: func(item map[string]any) map[string]any {
				return map[string]any{"networks": map[string]any{"default": map[string]any{"ipam": map[string]any{"config": []any{item}}}}}
			},
			scaffold: map[string]any{"networks": map[string]any{"default": map[string]any{}}},
		},
		{
			id: "develop.watch", path: []string{"services", "app", "develop", "watch"},
			identity: identity("path", "./src"),
			attrs: []itemAttr{
				{name: "action", base: "sync", user: "sync+restart", remote: "rebuild"},
				{name: "target", base: "/app", user: "/app/src", remote: "/app/dist"},
			},
			wholeList: true,
			wrap: func(item map[string]any) map[string]any {
				return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "develop": map[string]any{"watch": []any{item}}}}}
			},
			scaffold: serviceScaffold,
		},
		{
			id: "service.depends-on", path: []string{"services", "app", "depends_on"},
			identity: identity(),
			attrs: []itemAttr{
				{name: "condition", base: "service_started", user: "service_healthy", remote: "service_completed_successfully"},
				{name: "required", base: true, user: false, remote: true},
			},
			wrap: func(item map[string]any) map[string]any {
				return map[string]any{"services": map[string]any{"app": map[string]any{"image": "busybox:latest", "depends_on": map[string]any{"default": item}}}}
			},
			scaffold: serviceScaffold,
		},
	}
}

// --- multi-item scenarios --------------------------------------------------

// multiItemCases uses the third logical item so a field can hold three
// elements at once and only one or several of them change. This checks that
// untouched elements survive next to modified, deleted or untouched siblings.
func multiItemCases() ([]Case, error) {
	absent := Atom{}
	base := Atom{Present: true, Value: 0}
	user := Atom{Present: true, Value: 1}
	remote := Atom{Present: true, Value: 2}
	type itemState struct {
		first, second, third Atom
	}
	scenarios := []struct {
		name        string
		description string
		base        itemState
		user        itemState
		remote      itemState
	}{
		{
			"01-user-modifies-first-remote-modifies-second",
			"user changes the first element while newbase changes the second; the third stays untouched",
			itemState{base, base, base}, itemState{user, base, base}, itemState{base, remote, base},
		},
		{
			"02-user-changes-first-and-third-remote-changes-second",
			"user changes the first and third elements while newbase changes the second",
			itemState{base, base, base}, itemState{user, base, user}, itemState{base, remote, base},
		},
		{
			"03-user-deletes-first-remote-modifies-second",
			"user deletes the first element while newbase changes the second",
			itemState{base, base, base}, itemState{absent, base, base}, itemState{base, remote, base},
		},
		{
			"04-user-modifies-first-remote-deletes-second",
			"user changes the first element while newbase deletes the second",
			itemState{base, base, base}, itemState{user, base, base}, itemState{base, absent, base},
		},
		{
			"05-user-modifies-second-remote-changes-all",
			"newbase changes all three elements while user only changes the second",
			itemState{base, base, base}, itemState{base, user, base}, itemState{remote, remote, remote},
		},
		{
			"06-both-add-third-element-user-wins",
			"both sides add the third element with different values; user wins while existing elements follow newbase",
			itemState{base, base, absent}, itemState{base, base, user}, itemState{base, remote, remote},
		},
	}
	var cases []Case
	for _, field := range ArrayFields() {
		if field.Class == ClassAtomic {
			continue
		}
		values := [3][3]any{field.Values, field.SecondValues, field.ThirdValues}
		for _, scenario := range scenarios {
			states := [3]Atom{scenario.base.first, scenario.base.second, scenario.base.third}
			users := [3]Atom{scenario.user.first, scenario.user.second, scenario.user.third}
			remotes := [3]Atom{scenario.remote.first, scenario.remote.second, scenario.remote.third}
			baseValues := joinItems(values, states)
			userValues := joinItems(values, users)
			remoteValues := joinItems(values, remotes)
			var expectedValues []any
			if field.PairSemantics == PairAsWholeList {
				if slicesEqual(userValues, baseValues) {
					expectedValues = remoteValues
				} else {
					expectedValues = userValues
				}
			} else {
				for index := range values {
					expectedValues = append(expectedValues, atomValues(Resolve(states[index], users[index], remotes[index]), values[index])...)
				}
			}
			oldRoot, err := completeRoot(field, baseValues)
			if err != nil {
				return nil, err
			}
			userRoot, err := completeRoot(field, userValues)
			if err != nil {
				return nil, err
			}
			newRoot, err := completeRoot(field, remoteValues)
			if err != nil {
				return nil, err
			}
			expectedRoot, err := completeRoot(field, expectedValues)
			if err != nil {
				return nil, err
			}
			id := filepath.Join("multi-item", string(field.Class), field.ID, scenario.name)
			generated, err := renderExplicitCase(id, joinPath(field.ResetBoundary), scenario.name, scenario.description, [][]string{field.ResetBoundary}, oldRoot, userRoot, newRoot, expectedRoot)
			if err != nil {
				return nil, fmt.Errorf("generate %s: %w", id, err)
			}
			cases = append(cases, generated)
		}
	}
	return cases, nil
}

func joinItems(values [3][3]any, atoms [3]Atom) []any {
	var result []any
	for index, atom := range atoms {
		result = append(result, atomValues(atom, values[index])...)
	}
	return result
}

// ExplicitCaseCount returns the number of curated cases appended by Generate in
// addition to the per-field state matrix.
func ExplicitCaseCount() int {
	count := len(portAcceptanceFixtures())
	resourceCases, _ := multiResourceCases()
	count += len(resourceCases)
	attributeCases, _ := multiAttributeCases()
	count += len(attributeCases)
	itemCases, _ := multiItemCases()
	return count + len(itemCases)
}
