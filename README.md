# YAML Three-Way Merge

**English** | [简体中文](README.zh-CN.md)

A composable Go library and validation suite for semantic three-way merging of Compose YAML configurations.

The core problem is **configuration evolution under concurrent customization**: given a previous upstream configuration, a user override relative to that configuration, and a new upstream configuration, construct an effective configuration that preserves user changes while incorporating independent upstream changes. This requires distinguishing unchanged values, modifications, additions, and deletions, then resolving overlapping changes according to an explicit policy.

Unlike a textual merge, a semantic merge must account for equivalent syntactic representations, field-specific merge granularity, and element identity across versions. A mapping may represent independently editable keys or an atomic value; a sequence may represent an ordered value or a collection of identifiable items. Overrides further encode operations rather than complete states, so absence and explicit deletion cannot be interpreted interchangeably. When element identity or deletion scope is not recoverable from the inputs, the intended result may be underdetermined.

```text
previous base + user override + target base → effective configuration
```

The library separates input compilation, change extraction, replay validation, and policy application into reusable stages. Its validation methodology compares implementations against independently generated user-intent expectations, using a finite matrix of behavioral states rather than deriving expected results from the algorithms being evaluated. The suite contains 28,630 cases for the pinned Compose model (`compose-go v1.20.2`) and `x-casaos` extension fields. This is behavioral coverage within the declared model, not a proof of correctness for arbitrary YAML; see [known boundaries](#known-boundaries).

## Quick start

### Requirements

- Go 1.24 or later, as declared in [go.mod](go.mod).
- The checked-in `fixtures/` directory for corpus tests and validation commands.

Run the regression suite from the repository root:

```bash
GOWORK=off go test -count=1 ./... -skip '^TestFixtureCorpusAgainstCurrentImplementation$'
GOWORK=off go vet ./...
```

The excluded test is a strict check of the current project implementation against all user expectations; it has known failures. The command-level tests still verify that implementation's recorded baseline.

Run the intent strategy and print its report:

```bash
GOWORK=off go run ./cmd/validate-smd-intent -fixtures fixtures -json
```

The SMD validators return exit code `1` for mismatches or merge errors, and `2` for argument, fixture-loading, or report-writing errors. A baseline report with known mismatches therefore returns a nonzero status.

## Go library

Import the public package:

```go
import "github.com/runui/yaml-three-way-merge/merge"
```

### Merge in one call

```go
result, err := merge.Merge(merge.Input{
    PreviousBase: oldYAML,
    UserOverride: overrideYAML,
    TargetBase:   newYAML,
})
if err != nil {
    return err
}
// result.YAML is the effective configuration.
// result.Report describes changes, conflicts, and replay validation.
```

`UserOverride` is a real override, including multi-document `!reset` operations—not a fully materialized user configuration.

### Compose the stages

```text
Compile → Scenario ── ApplyDirect() → YAML
              │
              └── Analyze() → Plan ── Report()
                                └── Apply() → Result
```

```go
scenario, err := merge.Compile(input)
if err != nil {
    return err
}
plan, err := scenario.Analyze()
if err != nil {
    return err
}
result, err := plan.Apply()
if err != nil {
    return err
}
```

| API | Responsibility |
| --- | --- |
| `Compile(input)` | Normalize syntax, correlate identities, and compile the override against the previous base. |
| `scenario.Analyze()` | Extract both deltas and verify that replay reproduces each side. |
| `plan.Report()` | Return a diagnostic snapshot before application. |
| `plan.Apply()` | Apply user intent to upstream changes and reconcile named origins. |
| `scenario.ApplyDirect()` | Apply the user delta directly, without intent removal promotion or named-origin reconciliation. |
| `Equivalent(left, right)` | Compare modeled Compose semantics rather than YAML bytes. |

See the runnable example in [merge/example_test.go](merge/example_test.go).

### API contracts

- Results are effective configurations, not overrides. Comments, formatting, and anchors are not preserved.
- A compiled scenario is bound to its three inputs: identity correlation depends on both bases. Plans cannot be retargeted to another base.
- Input buffers may be changed after `Compile` returns. Returned YAML and reports belong to the caller; modifying them does not affect subsequent calls.
- Scenarios and plans support sequential reuse. Concurrent use of the same instance is not guaranteed.
- Nil or zero-value stage objects return `ErrUninitialized`, detectable with `errors.Is`.
- `*merge.Error` exposes `Stage` (`compile`, `analyze`, `apply`, or `compare`) and unwraps its cause. Use `errors.As`; error text is not a stable contract.
- Report paths are diagnostics, not a replayable patch format. `OriginRewrites` is determined during application and appears in `Result.Report`.

## Merge semantics

Expectations describe the user's effective state relative to the previous base:

| User action | Expected result |
| --- | --- |
| Unchanged | Follow the target base. |
| Modified | Keep the user's value. |
| Deleted | Keep the deletion. |
| Added | Keep the user's addition. |
| No action on an upstream addition | Inherit the addition. |

These rules apply according to the field's model:

- **Logical items:** merge individual identified elements. Free key/value mappings use the key as the logical identity.
- **Ordered lists:** treat the entire list as one value; an unchanged user list follows upstream, otherwise the user list wins.
- **Atomic fields:** merge whole values, including scalar forms of `build` and `extends`. Mapping forms are exercised through their leaf paths.

The intent engine uses structural identities where available, including volume targets, paths, and port tuples. Some fields use a trailing origin token after the last `-`. Identity rules are field-specific: the current model also contains positional handling for IPAM entries and tokenless string correlation. These heuristics are not universal identity guarantees.

Removal handling preserves independent upstream siblings and restores full surviving items when partial writes would otherwise lose unchanged attributes. Semantic comparison treats empty lists/maps as equivalent to missing containers. Direct and intent output paths intentionally differ in empty-container cleanup.

## Architecture

```text
merge/                       Public API, input/output contracts, stage errors
cmd/                         CLI arguments, strategy selection, reports
internal/
  smdmodel/                  Shared typed model, schema, normalization, identities
  smdmerge/                  Direct user-delta strategy
  smdintent/                 Intent analysis, replay, scoped removals, reconciliation
  compose/                   YAML AST, Compose merge, semantic comparison
  rebase/                    Existing project override-rebase strategy
  corpus/                    Field registry, intent matrix, fixture generation/loading
  validation/                Case execution, summaries, regression gates
```

```text
public merge API → smdmerge / smdintent → smdmodel
validation → corpus / rebase → compose
```

The two SMD strategies are peers; neither depends on the other. Shared typed operations belong in `smdmodel`, while conflict and removal policies belong in the strategy packages. The model does not depend on fixtures or validation.

Expected fixture results are generated independently from the intent matrix in [internal/corpus/states.go](internal/corpus/states.go). Never use the strategy under test to generate its expected output.

## Validation commands

| Command | Strategy |
| --- | --- |
| `cmd/validate` | Project `RebaseRepositoryUpdate`, including idempotence checks. |
| `cmd/validate-smd` | Direct SMD delta application. |
| `cmd/validate-smd-intent` | Intent strategy through the public API, with dual replay and conflict reporting. |

```bash
GOWORK=off go run ./cmd/validate -fixtures fixtures -json
GOWORK=off go run ./cmd/validate-smd -fixtures fixtures -json
GOWORK=off go run ./cmd/validate-smd -fixtures fixtures -compare-project -json
GOWORK=off go run ./cmd/validate-smd-intent -fixtures fixtures -json
```

Both SMD commands accept `-case <case-id>` to inspect one fixture, including actual/expected output for a mismatch.

### Recorded baselines

| Strategy | Total | Matched | Mismatched | Errors |
| --- | ---: | ---: | ---: | ---: |
| Project rebase | 28,630 | 24,210 | 4,420 | 0 |
| Direct SMD | 28,630 | 27,573 | 1,057 | 0 |
| Intent SMD | 28,630 | 28,541 | 89 | 0 |

The project baseline also has zero non-idempotent cases. Each command's `main_test.go` locks its full-corpus baseline. When behavior changes, update both the tests and these tables in both languages.

### Targeted checks

```bash
# Fixture inventory and field registry
GOWORK=off go test -run 'TestFixtureManifestIsComplete|TestArrayFieldRegistryIsAuditable' ./internal/validation

# Intent regression gate
GOWORK=off go test -run TestFixtureCorpusAgainstIntentImplementation ./internal/validation

# Strict project expectations: currently has known failures
GOWORK=off go test -run TestFixtureCorpusAgainstCurrentImplementation ./internal/validation
```

The intent gate excludes 50 underdetermined cases and checks remaining mismatch counts per field against [internal/validation/known_boundaries.go](internal/validation/known_boundaries.go). It is a two-way ratchet: both increases and decreases fail until the budget is updated. Fixing a boundary requires tightening its budget, not changing the expectation to fit the algorithm.

## Fixture corpus

### Layout and coverage

```text
fixtures/
  manifest.yml
  <merge-class>/<field>/single/<state>/
  <merge-class>/<field>/pair/<state-a>__<state-b>/
  multi/
  multi-attribute/
```

Each case contains five YAML files:

| File | Meaning |
| --- | --- |
| `case.yml` | Case metadata. |
| `oldbase.yml` | Previous repository configuration. |
| `user.yml` | Real user override. |
| `newbase.yml` | Target repository configuration. |
| `expected.yml` | Independently generated effective result. |

The corpus contains:

- 259 base field entries: 66 array-like and 193 scalar/mapping entries.
- 284 entries after expanding 25 syntax variants.
- 15 single-item states per entry; 225 two-item combinations for each non-atomic entry.
- Six three-item scenarios per array-like entry and four multi-resource scenarios per field entry.
- Four multi-attribute scenarios for each of eight entries, plus seven fixed port acceptance scenarios.
- 28,630 cases in total, including 1,645 explicit scenarios.

The [manifest](fixtures/manifest.yml) records field counts and pair semantics (`logical-items`, `whole-list`, or `none`). Tests verify inventory, required files, declared override intent, reset scope, and conformance of base/expected documents to the pinned Compose JSON Schema.

Fixtures are field-level fragments, not installable projects. Referenced resources may be absent and file paths symbolic. Exhaustiveness refers to behavioral state combinations, not every possible value, list length, or YAML representation.

### Syntax variants

`internal/corpus/variants_*.go` registers `FieldVariant` entries with a `Render` function and optional `ResetBoundary`. `ArrayFields()` expands them to IDs such as `<fieldID>@<variant>` while preserving pair semantics.

Variants cover short ports/volumes, 14 key/value map forms, string commands/entrypoints/health checks, scalar list forms, and named-map forms for `depends_on`/`networks`. Device mapping syntax is not registered because it is invalid for the pinned Compose version. Bare config/secret names are not syntax variants of the same identity model because they conflate source and target.

### Regeneration

```bash
GOWORK=off go run ./cmd/generate-fixtures -output fixtures
```

This rebuilds the entire `fixtures` directory. Runtime tests read the on-disk YAML. Resets target the field or its nearest reconstructible owner sequence; reset-and-write operations use separate YAML documents to avoid resetting unrelated resources.

## Known boundaries

### Underdetermined expectations

The intent gate explicitly excludes 50 cases through `knownUnsupportedIntent`:

- **42 IPAM cases:** `subnet` is both the only observable identity and the changed value. The inputs do not establish whether concurrent additions represent the same logical item or independent items.
- **8 network aliases/link-local-IP cases:** removing nested attributes is expected to remove the whole attachment, but that intention is not observable from the documents alone.

These cases remain in the corpus and full CLI totals. Resolving them requires revised fixture semantics or information beyond the three documents.

### Observable algorithm limitations

The intent strategy has 89 remaining mismatches: the 50 unobservable cases above and 39 unresolved boundaries:

- 10 identity-association cases involving generic-resource kinds, port `host_ip`/`protocol`, and positional IPAM identity.
- 1 long-form `depends_on` case requiring restoration of a complete named-map owner deleted upstream.
- 28 device options / IPAM aux_addresses cases involving nested-key versus owner deletion or restoration scope. Some overrides reset the entire owner while expecting independent upstream keys within it to survive; converting every list-item deletion into leaf deletion is insufficient.

General fixes now provide key-level free mappings, attribute-level structured mappings, normalization of both deploy label syntaxes, preservation of nested long-form volume attributes, and restoration of the outermost missing owner for nested list writes. These resolve 1,111 mismatches without changing fixture expectations. `ulimits` merge by name while each limit value remains atomic; unspecified fields retain atomic semantics.

Per-field budgets in `known_boundaries.go` are the executable record of these limitations. Fixture expectations remain the reference for improvements.
