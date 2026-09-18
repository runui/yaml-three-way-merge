package smdintent

import (
	"fmt"
	"sort"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/smdmerge"
	"sigs.k8s.io/structured-merge-diff/v7/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v7/typed"
)

type delta struct {
	added    *fieldpath.Set
	modified *fieldpath.Set
	removed  *fieldpath.Set
	writes   *fieldpath.Set
	source   *typed.TypedValue
}

func Merge(baseOldYAML, userOverrideYAML, baseNewYAML []byte) (Result, error) {
	scenario, err := smdmerge.CompileTypedScenario(baseOldYAML, userOverrideYAML, baseNewYAML)
	if err != nil {
		return Result{}, err
	}
	userDelta, err := buildDelta(scenario.Old, scenario.UserPrediction)
	if err != nil {
		return Result{}, fmt.Errorf("build user intent: %w", err)
	}
	upstreamDelta, err := buildDelta(scenario.Old, scenario.New)
	if err != nil {
		return Result{}, fmt.Errorf("build upstream intent: %w", err)
	}

	userReplay, err := applyDelta(scenario.Old, userDelta)
	if err != nil {
		return Result{}, fmt.Errorf("replay user intent: %w", err)
	}
	userValid, err := typedEqual(userReplay, scenario.UserPrediction)
	if err != nil {
		return Result{}, fmt.Errorf("validate user replay: %w", err)
	}
	if !userValid {
		return Result{}, fmt.Errorf("user intent replay does not reproduce user prediction")
	}

	upstreamReplay, err := applyDelta(scenario.Old, upstreamDelta)
	if err != nil {
		return Result{}, fmt.Errorf("replay upstream intent: %w", err)
	}
	upstreamValid, err := typedEqual(upstreamReplay, scenario.New)
	if err != nil {
		return Result{}, fmt.Errorf("validate upstream replay: %w", err)
	}
	if !upstreamValid {
		return Result{}, fmt.Errorf("upstream intent replay does not reproduce newbase")
	}

	final, err := applyDelta(upstreamReplay, userDelta)
	if err != nil {
		return Result{}, fmt.Errorf("apply user intent to upstream expectation: %w", err)
	}
	content, err := smdmerge.MarshalTypedValue(final)
	if err != nil {
		return Result{}, err
	}
	report := buildReport(userDelta, upstreamDelta)
	userYAML, err := smdmerge.MarshalTypedValue(scenario.UserPrediction)
	if err != nil {
		return Result{}, fmt.Errorf("marshal user prediction: %w", err)
	}
	content, report.OriginRewrites, err = reconcileNamedOrigins(baseOldYAML, userYAML, baseNewYAML, content)
	if err != nil {
		return Result{}, fmt.Errorf("reconcile named origins: %w", err)
	}
	report.UserReplayValid = userValid
	report.UpstreamReplayValid = upstreamValid
	return Result{YAML: content, Report: report}, nil
}

func Equivalent(leftYAML, rightYAML []byte) (bool, error) {
	return smdmerge.Equivalent(leftYAML, rightYAML)
}

func buildDelta(oldValue, sideValue *typed.TypedValue) (delta, error) {
	comparison, err := oldValue.Compare(sideValue)
	if err != nil {
		return delta{}, err
	}
	writes := comparison.Modified.Union(comparison.Added)
	writes, err = expandToValueLeaves(writes, sideValue)
	if err != nil {
		return delta{}, err
	}
	return delta{
		added: comparison.Added.Copy(), modified: comparison.Modified.Copy(),
		removed: comparison.Removed.Copy(), writes: writes, source: sideValue,
	}, nil
}

func applyDelta(base *typed.TypedValue, change delta) (*typed.TypedValue, error) {
	result := base.RemoveItems(change.removed)
	if change.writes.Empty() {
		return result, nil
	}
	patch := change.source.ExtractItems(change.writes, typed.WithAppendKeyFields())
	return result.Merge(patch)
}

func expandToValueLeaves(paths *fieldpath.Set, value *typed.TypedValue) (*fieldpath.Set, error) {
	valueFields, err := value.ToFieldSet()
	if err != nil {
		return nil, err
	}
	valueLeaves := valueFields.Leaves()
	result := fieldpath.NewSet()
	paths.Iterate(func(path fieldpath.Path) {
		matched := false
		valueLeaves.Iterate(func(candidate fieldpath.Path) {
			if pathPrefix(path, candidate) {
				result.Insert(candidate)
				matched = true
			}
		})
		if !matched {
			result.Insert(path)
		}
	})
	return result, nil
}

func typedEqual(left, right *typed.TypedValue) (bool, error) {
	comparison, err := left.Compare(right)
	return err == nil && comparison.IsSame(), err
}

func pathPrefix(prefix, path fieldpath.Path) bool {
	if len(prefix) > len(path) {
		return false
	}
	for index := range prefix {
		if !prefix[index].Equals(path[index]) {
			return false
		}
	}
	return true
}

func pathsOverlap(left, right fieldpath.Path) bool {
	return pathPrefix(left, right) || pathPrefix(right, left)
}

func buildReport(user, upstream delta) Report {
	report := Report{
		User: changeSummary(user), Upstream: changeSummary(upstream),
	}
	userWrites := setPaths(user.writes)
	userRemoves := setPaths(user.removed)
	upstreamWrites := setPaths(upstream.writes)
	upstreamRemoves := setPaths(upstream.removed)

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
