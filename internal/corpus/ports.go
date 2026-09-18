package corpus

import "fmt"

func portAcceptanceCases() ([]Case, error) {
	field := FieldSpec{ID: "service.ports", Path: []string{"services", "app", "ports"}, Class: ClassUniqueList}
	type fixture struct {
		id                        string
		old, user, next, expected []any
	}
	p := func(published string, target int) any { return fmt.Sprintf("%s:%d", published, target) }
	fixtures := []fixture{
		{"01-remote-modifies-untouched", []any{p("8081", 80)}, []any{p("8081", 80)}, []any{p("8082", 80)}, []any{p("8082", 80)}},
		{"02-user-and-remote-modify-user-wins", []any{p("8081", 80)}, []any{p("8082", 82)}, []any{p("8082", 80)}, []any{p("8082", 82)}},
		{"03-user-modifies-second-remote-deletes-it", []any{p("8081", 80), p("8082", 81)}, []any{p("8081", 80), p("8083", 81)}, []any{p("8081", 80)}, []any{p("8081", 80), p("8083", 81)}},
		{"04-user-modifies-first-and-deletes-second", []any{p("8081", 80), p("8082", 81)}, []any{p("8083", 80)}, []any{p("8081", 80)}, []any{p("8083", 80)}},
		{"05-user-modifies-and-remote-adds", []any{p("8081", 80)}, []any{p("8083", 80)}, []any{p("8081", 80), p("8082", 81)}, []any{p("8083", 80), p("8082", 81)}},
		{"06-user-clears-all", []any{p("8081", 80), p("8082", 81)}, nil, []any{p("8081", 80), p("8082", 81)}, nil},
		{"07-user-clears-old-remote-adds", []any{p("8081", 80), p("8082", 81)}, nil, []any{p("8081", 80), p("8082", 81), p("8083", 82)}, []any{p("8083", 82)}},
	}
	cases := make([]Case, 0, len(fixtures))
	for _, fixture := range fixtures {
		id := "unique-list/service.ports/acceptance/" + fixture.id
		generated, err := renderCase(field, id, "user-acceptance", "user-provided expected behavior", fixture.old, fixture.user, fixture.next, fixture.expected)
		if err != nil {
			return nil, err
		}
		generated.Dir = id
		cases = append(cases, generated)
	}
	return cases, nil
}
