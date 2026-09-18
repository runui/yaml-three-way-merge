package validation

import (
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/internal/compose"
	packoverride "github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/override"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
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
	overrideYAML, err := packoverride.RebaseRepositoryUpdate(item.BaseOld, item.BaseNew, item.User, packoverride.RepositoryRebaseOptions{})
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
	secondOverride, err := packoverride.RebaseRepositoryUpdate(item.BaseNew, item.BaseNew, overrideYAML, packoverride.RepositoryRebaseOptions{})
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
