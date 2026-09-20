package main

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/runui/yaml-three-way-merge/internal/compose"
	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/runui/yaml-three-way-merge/internal/rebase"
	publicmerge "github.com/runui/yaml-three-way-merge/merge"
)

type Algorithm struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OutputType  string `json:"output_type"`
}

var algorithms = []Algorithm{
	{ID: "intent", Name: "Intent SMD", Description: "提取并回放双方变更，冲突时保留用户意图。", OutputType: "effective"},
	{ID: "direct", Name: "Direct SMD", Description: "将用户字段差异直接应用到新基线。", OutputType: "effective"},
	{ID: "rebase", Name: "Project Rebase", Description: "项目原有的递归三方合并与覆盖层重建。", OutputType: "override"},
}

type MergeRequest struct {
	Algorithm    string `json:"algorithm"`
	PreviousBase string `json:"previous_base"`
	UserOverride string `json:"user_override"`
	TargetBase   string `json:"target_base"`
	Expected     string `json:"expected,omitempty"`
	HasExpected  bool   `json:"-"`
}

type MergeResponse struct {
	PreviousEffective string              `json:"previous_effective"`
	NewOverride       string              `json:"new_override"`
	NewEffective      string              `json:"new_effective"`
	MatchesExpected   *bool               `json:"matches_expected,omitempty"`
	Diagnostics       *publicmerge.Report `json:"diagnostics,omitempty"`
}

type FixtureSummary struct {
	ID            string `json:"id"`
	Field         string `json:"field"`
	Class         string `json:"class"`
	Scenario      string `json:"scenario"`
	Description   string `json:"description"`
	PairSemantics string `json:"pair_semantics,omitempty"`
}

type FixtureDetail struct {
	Metadata     FixtureSummary `json:"metadata"`
	PreviousBase string         `json:"previous_base"`
	UserOverride string         `json:"user_override"`
	TargetBase   string         `json:"target_base"`
	Expected     string         `json:"expected"`
}

type FixturePage struct {
	Items    []FixtureSummary `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type Service struct {
	cases []corpus.Case
	byID  map[string]int
}

func NewService(fixturesRoot string) (*Service, error) {
	cases, _, err := corpus.Load(fixturesRoot)
	if err != nil {
		return nil, fmt.Errorf("load fixtures: %w", err)
	}
	byID := make(map[string]int, len(cases))
	for index, item := range cases {
		id := item.Metadata.ID
		if id == "" {
			return nil, fmt.Errorf("fixture %q has an empty id", item.Dir)
		}
		if id != strings.ReplaceAll(item.Dir, "\\", "/") {
			return nil, fmt.Errorf("fixture id %q does not match directory %q", id, item.Dir)
		}
		if _, duplicate := byID[id]; duplicate {
			return nil, fmt.Errorf("duplicate fixture id %q", id)
		}
		byID[id] = index
	}
	return &Service{cases: cases, byID: byID}, nil
}

func Algorithms() []Algorithm {
	return append([]Algorithm(nil), algorithms...)
}

func (s *Service) Merge(request MergeRequest) (MergeResponse, error) {
	if !knownAlgorithm(request.Algorithm) {
		return MergeResponse{}, &RequestError{Stage: "request", Err: fmt.Errorf("unknown algorithm %q", request.Algorithm)}
	}
	previousBase := []byte(request.PreviousBase)
	userOverride := []byte(request.UserOverride)
	targetBase := []byte(request.TargetBase)
	if len(bytes.TrimSpace(previousBase)) == 0 {
		return MergeResponse{}, &RequestError{Stage: "request", Err: errors.New("previous_base is required")}
	}
	if len(bytes.TrimSpace(targetBase)) == 0 {
		return MergeResponse{}, &RequestError{Stage: "request", Err: errors.New("target_base is required")}
	}

	previousEffective, err := compose.MergeYAML(previousBase, userOverride)
	if err != nil {
		return MergeResponse{}, &RequestError{Stage: "previous_effective", Err: err}
	}

	var newOverride, algorithmEffective []byte
	var diagnostics *publicmerge.Report
	switch request.Algorithm {
	case "intent":
		result, mergeErr := publicmerge.Merge(publicmerge.Input{PreviousBase: previousBase, UserOverride: userOverride, TargetBase: targetBase})
		if mergeErr != nil {
			return MergeResponse{}, mergeRequestError(mergeErr)
		}
		algorithmEffective = result.YAML
		report := result.Report
		diagnostics = &report
	case "direct":
		scenario, compileErr := publicmerge.Compile(publicmerge.Input{PreviousBase: previousBase, UserOverride: userOverride, TargetBase: targetBase})
		if compileErr != nil {
			return MergeResponse{}, mergeRequestError(compileErr)
		}
		algorithmEffective, err = scenario.ApplyDirect()
		if err != nil {
			return MergeResponse{}, mergeRequestError(err)
		}
	case "rebase":
		newOverride, err = rebase.RebaseRepositoryUpdate(previousBase, targetBase, userOverride, rebase.RepositoryRebaseOptions{})
		if err != nil {
			return MergeResponse{}, &RequestError{Stage: "rebase", Err: err}
		}
	}

	if request.Algorithm != "rebase" {
		newOverride, err = rebase.Build(targetBase, algorithmEffective)
		if err != nil {
			return MergeResponse{}, &RequestError{Stage: "build_override", Err: err}
		}
	}
	newEffective, err := compose.MergeYAML(targetBase, newOverride)
	if err != nil {
		return MergeResponse{}, &RequestError{Stage: "new_effective", Err: err}
	}
	if algorithmEffective != nil {
		equal, compareErr := compose.SemanticYAMLEqual(algorithmEffective, newEffective)
		if compareErr != nil {
			return MergeResponse{}, &RequestError{Stage: "verify_override", Err: compareErr}
		}
		if !equal {
			return MergeResponse{}, &RequestError{Stage: "verify_override", Err: errors.New("generated override does not reproduce the algorithm result")}
		}
	}

	response := MergeResponse{
		PreviousEffective: string(previousEffective),
		NewOverride:       string(newOverride),
		NewEffective:      string(newEffective),
		Diagnostics:       diagnostics,
	}
	if request.HasExpected {
		matches, compareErr := publicmerge.Equivalent(newEffective, []byte(request.Expected))
		if compareErr != nil {
			return MergeResponse{}, &RequestError{Stage: "compare_expected", Err: compareErr}
		}
		response.MatchesExpected = &matches
	}
	return response, nil
}

func (s *Service) ListFixtures(query, class string, page, pageSize int) FixturePage {
	query = strings.ToLower(strings.TrimSpace(query))
	class = strings.TrimSpace(class)
	matches := make([]FixtureSummary, 0)
	for _, item := range s.cases {
		summary := fixtureSummary(item)
		if class != "" && summary.Class != class {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(summary.ID+" "+summary.Field+" "+summary.Scenario+" "+summary.Description), query) {
			continue
		}
		matches = append(matches, summary)
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	total := len(matches)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(start+pageSize, total)
	return FixturePage{Items: matches[start:end], Total: total, Page: page, PageSize: pageSize}
}

func (s *Service) Fixture(id string) (FixtureDetail, bool) {
	index, ok := s.byID[id]
	if !ok {
		return FixtureDetail{}, false
	}
	item := s.cases[index]
	return FixtureDetail{
		Metadata:     fixtureSummary(item),
		PreviousBase: string(item.BaseOld),
		UserOverride: string(item.User),
		TargetBase:   string(item.BaseNew),
		Expected:     string(item.Expected),
	}, true
}

func fixtureSummary(item corpus.Case) FixtureSummary {
	return FixtureSummary{
		ID:            item.Metadata.ID,
		Field:         item.Metadata.Field,
		Class:         string(item.Metadata.Class),
		Scenario:      item.Metadata.Scenario,
		Description:   item.Metadata.Description,
		PairSemantics: string(item.Metadata.PairSemantics),
	}
}

func knownAlgorithm(id string) bool {
	for _, algorithm := range algorithms {
		if algorithm.ID == id {
			return true
		}
	}
	return false
}

type RequestError struct {
	Stage string
	Err   error
}

func (e *RequestError) Error() string { return fmt.Sprintf("%s: %v", e.Stage, e.Err) }
func (e *RequestError) Unwrap() error { return e.Err }

func mergeRequestError(err error) error {
	var mergeErr *publicmerge.Error
	if errors.As(err, &mergeErr) {
		return &RequestError{Stage: string(mergeErr.Stage), Err: mergeErr.Err}
	}
	return &RequestError{Stage: "merge", Err: err}
}
