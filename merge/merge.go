// Package merge provides composable Compose YAML three-way merge operations.
// Inputs contain a previous base, a real user override (including multi-document
// !reset operations), and a target base. Results are effective configurations,
// not overrides. Formatting, comments and anchors are not preserved.
package merge

import (
	"errors"
	"fmt"

	"github.com/runui/yaml-three-way-merge/internal/smdintent"
	"github.com/runui/yaml-three-way-merge/internal/smdmerge"
	"github.com/runui/yaml-three-way-merge/internal/smdmodel"
)

// Input names the three documents to avoid positional argument mistakes.
// Compile reads these buffers synchronously and retains its own copies.
type Input struct {
	PreviousBase []byte
	UserOverride []byte
	TargetBase   []byte
}

// Stage identifies a failing pipeline operation independently of error text.
type Stage string

const (
	StageCompile Stage = "compile"
	StageAnalyze Stage = "analyze"
	StageApply   Stage = "apply"
	StageCompare Stage = "compare"
)

// Error preserves the underlying cause for errors.Is and errors.As.
type Error struct {
	Stage Stage
	Err   error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %v", e.Stage, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// ErrUninitialized indicates a nil or zero-value Scenario or Plan.
var ErrUninitialized = errors.New("uninitialized merge stage")

// Scenario is an opaque compiled snapshot. Create one with Compile. It may be
// reused sequentially for either strategy. Concurrent use is not guaranteed.
type Scenario struct {
	value       smdmodel.TypedScenario
	old, target []byte
}

// Compile normalizes syntax, assigns identities and applies the override to the
// previous base. Identity correlation requires all three inputs together; a
// compiled scenario cannot be retargeted to an unrelated upstream document.
func Compile(input Input) (*Scenario, error) {
	value, err := smdmodel.CompileTypedScenario(input.PreviousBase, input.UserOverride, input.TargetBase)
	if err != nil {
		return nil, &Error{StageCompile, err}
	}
	return &Scenario{value: value, old: append([]byte(nil), input.PreviousBase...), target: append([]byte(nil), input.TargetBase...)}, nil
}

// Plan is replay-validated user and upstream intent, created by Analyze.
// Apply returns fresh output and report data on every call.
type Plan struct{ value *smdintent.Plan }

// Analyze extracts and replay-validates both sides without applying conflicts.
func (s *Scenario) Analyze() (*Plan, error) {
	if s == nil || s.value.Old == nil {
		return nil, &Error{StageAnalyze, ErrUninitialized}
	}
	plan, err := smdintent.Analyze(s.value, s.old, s.target)
	if err != nil {
		return nil, &Error{StageAnalyze, err}
	}
	return &Plan{value: plan}, nil
}

// ApplyDirect applies the user's field delta without intent removal promotion or
// named-origin reconciliation. It preserves the existing direct-SMD semantics.
func (s *Scenario) ApplyDirect() ([]byte, error) {
	if s == nil || s.value.Old == nil {
		return nil, &Error{StageApply, ErrUninitialized}
	}
	content, err := smdmerge.Apply(s.value)
	if err != nil {
		return nil, &Error{StageApply, err}
	}
	return content, nil
}

// Apply resolves conflicts in favor of user intent, preserving independent
// upstream changes. The returned YAML and report are owned by the caller.
func (p *Plan) Apply() (Result, error) {
	if p == nil || p.value == nil {
		return Result{}, &Error{StageApply, ErrUninitialized}
	}
	result, err := p.value.Apply()
	if err != nil {
		return Result{}, &Error{StageApply, err}
	}
	return Result{YAML: result.YAML, Report: publicReport(result.Report)}, nil
}

// Report returns a snapshot before application. OriginRewrites is zero until
// application; the final count is available in Result.Report.
func (p *Plan) Report() (Report, error) {
	if p == nil || p.value == nil {
		return Report{}, &Error{StageAnalyze, ErrUninitialized}
	}
	return publicReport(p.value.Report()), nil
}

// Merge is the convenience composition Compile → Analyze → Apply.
func Merge(input Input) (Result, error) {
	scenario, err := Compile(input)
	if err != nil {
		return Result{}, err
	}
	plan, err := scenario.Analyze()
	if err != nil {
		return Result{}, err
	}
	return plan.Apply()
}

// Equivalent compares modeled Compose semantics, including short/long syntax
// normalization and missing/empty-container equivalence, not YAML byte equality.
func Equivalent(left, right []byte) (bool, error) {
	equal, err := smdmodel.Equivalent(left, right)
	if err != nil {
		return false, &Error{StageCompare, err}
	}
	return equal, nil
}
