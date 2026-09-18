package validation

import (
	"fmt"

	"github.com/runui/yaml-three-way-merge/internal/compose"
	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/runui/yaml-three-way-merge/internal/rebase"
)

type Result struct {
	Case       corpus.Case
	Override   []byte
	Actual     []byte
	Matches    bool
	Idempotent bool
	Error      error
}

func Run(item corpus.Case) Result {
	result := Result{Case: item}
	overrideYAML, err := rebase.RebaseRepositoryUpdate(item.BaseOld, item.BaseNew, item.User, rebase.RepositoryRebaseOptions{})
	if err != nil {
		result.Error = fmt.Errorf("rebase: %w", err)
		return result
	}
	result.Override = overrideYAML
	actual, err := compose.MergeYAML(item.BaseNew, overrideYAML)
	if err != nil {
		result.Error = fmt.Errorf("merge result: %w", err)
		return result
	}
	result.Actual = actual
	result.Matches, err = compose.SemanticYAMLEqual(actual, item.Expected)
	if err != nil {
		result.Error = fmt.Errorf("compare expected: %w", err)
		return result
	}
	secondOverride, err := rebase.RebaseRepositoryUpdate(item.BaseNew, item.BaseNew, overrideYAML, rebase.RepositoryRebaseOptions{})
	if err != nil {
		result.Error = fmt.Errorf("rebase idempotence: %w", err)
		return result
	}
	secondActual, err := compose.MergeYAML(item.BaseNew, secondOverride)
	if err != nil {
		result.Error = fmt.Errorf("merge idempotence: %w", err)
		return result
	}
	result.Idempotent, err = compose.SemanticYAMLEqual(actual, secondActual)
	if err != nil {
		result.Error = fmt.Errorf("compare idempotence: %w", err)
	}
	return result
}
