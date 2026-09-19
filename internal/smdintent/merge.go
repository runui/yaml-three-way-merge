package smdintent

import (
	"fmt"
	"sort"

	"github.com/runui/yaml-three-way-merge/internal/smdmodel"
	"sigs.k8s.io/structured-merge-diff/v7/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v7/typed"
)

type delta struct {
	added          *fieldpath.Set
	modified       *fieldpath.Set
	removed        *fieldpath.Set
	intentRemovals *fieldpath.Set
	writes         *fieldpath.Set
	source         *typed.TypedValue
}

func Merge(baseOldYAML, userOverrideYAML, baseNewYAML []byte) (Result, error) {
	scenario, err := smdmodel.CompileTypedScenario(baseOldYAML, userOverrideYAML, baseNewYAML)
	if err != nil {
		return Result{}, err
	}
	plan, err := Analyze(scenario, baseOldYAML, baseNewYAML)
	if err != nil {
		return Result{}, err
	}
	return plan.Apply()
}

// Plan owns replay-validated intent and can be applied repeatedly. Typed values
// remain private; serializers copy their containers before removing model fields.
type Plan struct {
	user             delta
	upstream         *typed.TypedValue
	prediction       *typed.TypedValue
	oldYAML, newYAML []byte
	report           Report
}

// Analyze extracts both deltas and verifies they reproduce the compiled states.
// Raw bases are retained because named-origin reconciliation uses authored names.
func Analyze(scenario smdmodel.TypedScenario, baseOldYAML, baseNewYAML []byte) (*Plan, error) {
	userDelta, err := buildDelta(scenario.Old, scenario.UserPrediction)
	if err != nil {
		return nil, fmt.Errorf("build user intent: %w", err)
	}
	upstreamDelta, err := buildDelta(scenario.Old, scenario.New)
	if err != nil {
		return nil, fmt.Errorf("build upstream intent: %w", err)
	}

	userReplay, err := applyDelta(scenario.Old, userDelta)
	if err != nil {
		return nil, fmt.Errorf("replay user intent: %w", err)
	}
	userValid, err := typedEqual(userReplay, scenario.UserPrediction)
	if err != nil {
		return nil, fmt.Errorf("validate user replay: %w", err)
	}
	if !userValid {
		replayYAML, _ := smdmodel.MarshalTypedValue(userReplay)
		predictionYAML, _ := smdmodel.MarshalTypedValue(scenario.UserPrediction)
		return nil, fmt.Errorf("user intent replay does not reproduce user prediction\nreplay:\n%s\nprediction:\n%s", replayYAML, predictionYAML)
	}

	upstreamReplay, err := applyDelta(scenario.Old, upstreamDelta)
	if err != nil {
		return nil, fmt.Errorf("replay upstream intent: %w", err)
	}
	upstreamValid, err := typedEqual(upstreamReplay, scenario.New)
	if err != nil {
		return nil, fmt.Errorf("validate upstream replay: %w", err)
	}
	if !upstreamValid {
		return nil, fmt.Errorf("upstream intent replay does not reproduce newbase")
	}
	report := buildReport(userDelta, upstreamDelta)
	report.UserReplayValid = userValid
	report.UpstreamReplayValid = upstreamValid
	return &Plan{user: userDelta, upstream: upstreamReplay, prediction: scenario.UserPrediction,
		oldYAML: append([]byte(nil), baseOldYAML...), newYAML: append([]byte(nil), baseNewYAML...), report: report}, nil
}

// Apply resolves user intent against upstream, then reconciles named resources.
func (p *Plan) Apply() (Result, error) {
	final, err := applyIntent(p.upstream, p.user)
	if err != nil {
		return Result{}, fmt.Errorf("apply user intent to upstream expectation: %w", err)
	}
	content, err := smdmodel.MarshalTypedValue(final)
	if err != nil {
		return Result{}, err
	}
	report := p.Report()
	userYAML, err := smdmodel.MarshalTypedValue(p.prediction)
	if err != nil {
		return Result{}, fmt.Errorf("marshal user prediction: %w", err)
	}
	content, report.OriginRewrites, err = reconcileNamedOrigins(p.oldYAML, userYAML, p.newYAML, content)
	if err != nil {
		return Result{}, fmt.Errorf("reconcile named origins: %w", err)
	}
	return Result{YAML: content, Report: report}, nil
}

// Report returns an independent snapshot; origin rewrites are populated by Apply.
func (p *Plan) Report() Report {
	report := p.report
	report.User.Paths = append([]string(nil), report.User.Paths...)
	report.Upstream.Paths = append([]string(nil), report.Upstream.Paths...)
	return report
}

func Equivalent(leftYAML, rightYAML []byte) (bool, error) {
	return smdmodel.Equivalent(leftYAML, rightYAML)
}

func buildDelta(oldValue, sideValue *typed.TypedValue) (delta, error) {
	comparison, err := oldValue.Compare(sideValue)
	if err != nil {
		return delta{}, err
	}
	writes := comparison.Modified.Union(comparison.Added)
	writes, err = smdmodel.ExpandToValueLeaves(writes, sideValue)
	if err != nil {
		return delta{}, err
	}
	intentRemovals, err := intentRemovalPaths(comparison.Removed, sideValue)
	if err != nil {
		return delta{}, err
	}
	return delta{
		added: comparison.Added.Copy(), modified: comparison.Modified.Copy(),
		removed: comparison.Removed.Copy(), intentRemovals: intentRemovals,
		writes: writes, source: sideValue,
	}, nil
}

func applyDelta(base *typed.TypedValue, change delta) (*typed.TypedValue, error) {
	result := base.RemoveItems(change.removed)
	return applyWrites(result, change)
}

func applyIntent(base *typed.TypedValue, change delta) (*typed.TypedValue, error) {
	result := base.RemoveItems(change.intentRemovals)
	return applyWrites(result, change)
}

func applyWrites(result *typed.TypedValue, change delta) (*typed.TypedValue, error) {
	if change.writes.Empty() {
		return result, nil
	}
	writes := change.writes
	// A partial write inside a list item whose entry was deleted upstream would
	// re-create the item with only the changed fields. Restore the full item in
	// that case; the target has no entry, so no independent upstream sibling can
	// be overwritten.
	extra := fieldpath.NewSet()
	sourceLeaves, err := change.source.ToFieldSet()
	if err != nil {
		return nil, err
	}
	leaves := sourceLeaves.Leaves()
	// The target is unchanged throughout this scan. Build its field set once,
	// rather than re-traversing the whole document for every candidate write.
	targetFields, err := result.ToFieldSet()
	if err != nil {
		return nil, err
	}
	targetLeaves := targetFields.Leaves()
	change.writes.Iterate(func(path fieldpath.Path) {
		item := path
		// Restore the outermost missing item. A nested keyed attribute may
		// itself be absent because its entire owner was deleted upstream.
		for index, element := range path {
			if element.Key != nil && !hasLeafBelow(targetLeaves, path[:index+1]) {
				item = path[:index+1]
				break
			}
		}
		if len(item) >= len(path) {
			return
		}
		if hasLeafBelow(targetLeaves, item) {
			return
		}
		leaves.Iterate(func(leaf fieldpath.Path) {
			if smdmodel.PathPrefix(item, leaf) {
				extra.Insert(leaf)
			}
		})
	})
	patch := change.source.ExtractItems(writes, typed.WithAppendKeyFields())
	merged, err := result.Merge(patch)
	if err != nil {
		return nil, err
	}
	if extra.Empty() {
		return merged, nil
	}
	// Re-add the full missing items so unchanged sibling fields survive.
	extraPatch := change.source.ExtractItems(extra, typed.WithAppendKeyFields())
	return merged.Merge(extraPatch)
}

// enclosingItemPath returns the path of the nearest enclosing associative item,
// i.e. the prefix ending at the last keyed element. Atomic sequence indices are
// not item boundaries: removing one index leaves a sparse list rather than
// removing a logical element.
func enclosingItemPath(path fieldpath.Path) fieldpath.Path {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index].Key != nil {
			return path[:index+1]
		}
	}
	return path
}

// intentRemovalPaths promotes a nested removal to its enclosing keyed list item,
// unless the side value still contains that item. In that case the precise child
// path is kept so independent upstream siblings inside the same item survive.
// A parent path is dropped when a more precise descendant is also present.
func intentRemovalPaths(paths *fieldpath.Set, source *typed.TypedValue) (*fieldpath.Set, error) {
	var candidates []fieldpath.Path
	var err error
	paths.Iterate(func(path fieldpath.Path) {
		if err != nil {
			return
		}
		chosen, chooseErr := removedItemPath(path, source)
		if chooseErr != nil {
			err = chooseErr
			return
		}
		candidates = append(candidates, chosen)
	})
	if err != nil {
		return nil, err
	}
	result := fieldpath.NewSet()
	for index, item := range candidates {
		hasDescendant := false
		for otherIndex, other := range candidates {
			if index == otherIndex || len(other) <= len(item) {
				continue
			}
			if smdmodel.PathPrefix(item, other) {
				hasDescendant = true
				break
			}
		}
		if !hasDescendant {
			result.Insert(item)
		}
	}
	return result, nil
}

// removedItemPath chooses the outermost keyed list item that no longer exists in
// the side value. For a removed item that is the whole element; for a nested
// removal that kept its element, it is the nested set item. When every enclosing
// element survives, the full path is kept.
func removedItemPath(path fieldpath.Path, source *typed.TypedValue) (fieldpath.Path, error) {
	for index, element := range path {
		if element.Key == nil {
			continue
		}
		prefix := path[:index+1]
		survives, err := pathSurvives(source, prefix)
		if err != nil {
			return nil, err
		}
		if !survives {
			return prefix.Copy(), nil
		}
	}
	return path.Copy(), nil
}

// pathSurvives reports whether the value still has any leaf below path. It uses
// the field set rather than ExtractItems because extracting an absent path still
// returns the ancestor scaffolding.
func pathSurvives(value *typed.TypedValue, path fieldpath.Path) (bool, error) {
	fields, err := value.ToFieldSet()
	if err != nil {
		return false, err
	}
	return hasLeafBelow(fields.Leaves(), path), nil
}

func hasLeafBelow(leaves *fieldpath.Set, path fieldpath.Path) bool {
	survives := false
	leaves.Iterate(func(leaf fieldpath.Path) {
		if !survives && smdmodel.PathPrefix(path, leaf) {
			survives = true
		}
	})
	return survives
}

func typedEqual(left, right *typed.TypedValue) (bool, error) {
	leftYAML, err := smdmodel.MarshalTypedValue(left)
	if err != nil {
		return false, err
	}
	rightYAML, err := smdmodel.MarshalTypedValue(right)
	if err != nil {
		return false, err
	}
	return smdmodel.Equivalent(leftYAML, rightYAML)
}

func pathsOverlap(left, right fieldpath.Path) bool {
	return smdmodel.PathPrefix(left, right) || smdmodel.PathPrefix(right, left)
}

func buildReport(user, upstream delta) Report {
	report := Report{
		User: changeSummary(user), Upstream: changeSummary(upstream),
	}
	userWrites := setPaths(user.writes)
	userRemoves := setPaths(user.intentRemovals)
	upstreamWrites := setPaths(upstream.writes)
	upstreamRemoves := setPaths(upstream.intentRemovals)

	userTouched := append(append([]fieldpath.Path{}, userWrites...), userRemoves...)
	upstreamTouched := append(append([]fieldpath.Path{}, upstreamWrites...), upstreamRemoves...)
	for _, path := range userTouched {
		if !overlapsAny(path, upstreamTouched) {
			report.IndependentUser++
		}
	}
	for _, path := range upstreamTouched {
		if !overlapsAny(path, userTouched) {
			report.IndependentUpstream++
		}
	}
	for _, userPath := range userWrites {
		for _, upstreamPath := range upstreamWrites {
			if pathsOverlap(userPath, upstreamPath) {
				report.Conflicts.WriteWrite++
			}
		}
		for _, upstreamPath := range upstreamRemoves {
			if pathsOverlap(userPath, upstreamPath) {
				report.Conflicts.WriteRemove++
			}
		}
	}
	for _, userPath := range userRemoves {
		for _, upstreamPath := range upstreamWrites {
			if pathsOverlap(userPath, upstreamPath) {
				report.Conflicts.RemoveWrite++
			}
		}
		for _, upstreamPath := range upstreamRemoves {
			if pathsOverlap(userPath, upstreamPath) {
				report.Conflicts.BothRemove++
			}
		}
	}
	return report
}

func changeSummary(change delta) ChangeSummary {
	added := logicalPaths(change.added)
	modified := logicalPaths(change.modified)
	removed := logicalPaths(change.removed)
	paths := append(append(append([]string{}, added...), modified...), removed...)
	sort.Strings(paths)
	return ChangeSummary{
		Added: len(added), Modified: len(modified), Removed: len(removed), Paths: paths,
	}
}

func setPaths(set *fieldpath.Set) []fieldpath.Path {
	var paths []fieldpath.Path
	set.Iterate(func(path fieldpath.Path) { paths = append(paths, path.Copy()) })
	return paths
}

func logicalPaths(set *fieldpath.Set) []string {
	unique := map[string]struct{}{}
	set.Iterate(func(path fieldpath.Path) {
		logical := path
		for index, element := range path {
			if element.Key != nil || element.Value != nil {
				logical = path[:index+1]
				break
			}
		}
		unique[logical.String()] = struct{}{}
	})
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func overlapsAny(path fieldpath.Path, candidates []fieldpath.Path) bool {
	for _, candidate := range candidates {
		if pathsOverlap(path, candidate) {
			return true
		}
	}
	return false
}
