package corpus

import (
	"bytes"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Generate() ([]Case, Manifest, error) {
	states := FifteenStates()
	fields := ArrayFields()
	cases := make([]Case, 0, len(fields)*len(states)*len(states))
	manifest := Manifest{
		ComposeGoVersion: "v1.20.2",
		Policy:           "user changes relative to oldbase win according to each field's pair semantics; untouched values follow newbase",
		FilesPerCase:     []string{"case.yml", "oldbase.yml", "user.yml", "newbase.yml", "expected.yml"},
		StateCount:       len(states),
	}
	for _, field := range fields {
		entry := ManifestField{ID: field.ID, Path: joinPath(field.Path), Class: field.Class, MatrixCases: len(states), PairSemantics: field.PairSemantics}
		for _, state := range states {
			base := atomValues(state.Base, field.Values)
			user := atomValues(state.User, field.Values)
			remote := atomValues(state.Remote, field.Values)
			expected := atomValues(state.Expected, field.Values)
			id := filepath.Join(string(field.Class), field.ID, "single", state.ID)
			generated, err := renderCase(field, id, state.ID, stateDescription(state), base, user, remote, expected)
			if err != nil {
				return nil, Manifest{}, fmt.Errorf("generate %s: %w", id, err)
			}
			generated.Dir = id
			cases = append(cases, generated)
		}
		if field.CrossProduct {
			entry.CrossCases = len(states) * len(states)
			for _, first := range states {
				for _, second := range states {
					base := append(atomValues(first.Base, field.Values), atomValues(second.Base, field.SecondValues)...)
					user := append(atomValues(first.User, field.Values), atomValues(second.User, field.SecondValues)...)
					remote := append(atomValues(first.Remote, field.Values), atomValues(second.Remote, field.SecondValues)...)
					var expected []any
					if field.PairSemantics == PairAsWholeList {
						if slicesEqual(user, base) {
							expected = remote
						} else {
							expected = user
						}
					} else {
						expected = append(atomValues(first.Expected, field.Values), atomValues(second.Expected, field.SecondValues)...)
					}
					id := filepath.Join(string(field.Class), field.ID, "pair", first.ID+"__"+second.ID)
					generated, err := renderCase(field, id, "pair-cross-product", first.ID+" combined with "+second.ID, base, user, remote, expected)
					if err != nil {
						return nil, Manifest{}, fmt.Errorf("generate %s: %w", id, err)
					}
					generated.Dir = id
					cases = append(cases, generated)
				}
			}
		}
		manifest.Fields = append(manifest.Fields, entry)
	}
	explicit, err := portAcceptanceCases()
	if err != nil {
		return nil, Manifest{}, err
	}
	cases = append(cases, explicit...)
	resourceCases, err := multiResourceCases()
	if err != nil {
		return nil, Manifest{}, err
	}
	cases = append(cases, resourceCases...)
	attributeCases, err := multiAttributeCases()
	if err != nil {
		return nil, Manifest{}, err
	}
	cases = append(cases, attributeCases...)
	itemCases, err := multiItemCases()
	if err != nil {
		return nil, Manifest{}, err
	}
	cases = append(cases, itemCases...)
	manifest.ExplicitCases = len(explicit) + len(resourceCases) + len(attributeCases) + len(itemCases)
	manifest.CaseCount = len(cases)
	return cases, manifest, nil
}

func atomValues(atom Atom, values [3]any) []any {
	if !atom.Present {
		return nil
	}
	return []any{values[atom.Value]}
}

func slicesEqual(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		leftYAML, _ := yaml.Marshal(left[index])
		rightYAML, _ := yaml.Marshal(right[index])
		if !bytes.Equal(leftYAML, rightYAML) {
			return false
		}
	}
	return true
}

func stateDescription(state State) string {
	if state.User == state.Base {
		return "user is untouched, so the logical item follows newbase"
	}
	return "user changed the logical item relative to oldbase, so user intent wins"
}
