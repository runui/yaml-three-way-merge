package corpus

type MergeClass string

type PairSemantics string

const (
	PairByLogicalItem PairSemantics = "logical-items"
	PairAsWholeList   PairSemantics = "whole-list"
	PairDisabled      PairSemantics = "none"
)

const (
	ClassScalar      MergeClass = "scalar"
	ClassMappingItem MergeClass = "mapping-item"
	ClassNamedItem   MergeClass = "named-item"
	ClassUniqueList  MergeClass = "unique-list"
	ClassSetList     MergeClass = "set-list"
	ClassOrderedList MergeClass = "ordered-list"
	ClassAtomic      MergeClass = "atomic"
	ClassPolymorphic MergeClass = "polymorphic"
)

type FieldSpec struct {
	ID            string
	Path          []string
	Class         MergeClass
	Values        [3]any
	SecondValues  [3]any
	ThirdValues   [3]any
	CrossProduct  bool
	PairSemantics PairSemantics
	ResetBoundary []string
	Wrap          func([]any, bool) map[string]any
	// Render, when set, encodes the logical values into a root fragment using a
	// particular syntax variant. It takes precedence over Wrap and Path.
	Render func(FieldSpec, []any, bool) map[string]any
	// Variant names the syntax variant this spec encodes. Empty is the default.
	Variant string
}

// FieldVariant registers an alternative, syntactically valid encoding of the
// same logical field. The merge semantics are unchanged; only the YAML shape
// differs (for example short vs long syntax, list vs mapping, scalar vs
// sequence).
type FieldVariant struct {
	Name          string
	Render        func(FieldSpec, []any, bool) map[string]any
	ResetBoundary []string
}

type State struct {
	ID       string
	Base     Atom
	User     Atom
	Remote   Atom
	Expected Atom
}

type Atom struct {
	Present bool
	Value   int
}

type Metadata struct {
	ID            string        `yaml:"id"`
	Field         string        `yaml:"field"`
	Class         MergeClass    `yaml:"class"`
	Scenario      string        `yaml:"scenario"`
	Description   string        `yaml:"description"`
	PairSemantics PairSemantics `yaml:"pair_semantics,omitempty"`
}

type Case struct {
	Metadata      Metadata
	Dir           string
	BaseOld       []byte
	User          []byte
	BaseNew       []byte
	Expected      []byte
	UserEffective []byte
}

type Manifest struct {
	ComposeGoVersion string          `yaml:"compose_go_version"`
	Policy           string          `yaml:"policy"`
	FilesPerCase     []string        `yaml:"files_per_case"`
	StateCount       int             `yaml:"state_count"`
	Fields           []ManifestField `yaml:"fields"`
	// ExplicitCases counts curated scenarios that are not part of the
	// per-field state matrix: port acceptance, multi-resource, multi-attribute
	// and multi-item families.
	ExplicitCases int `yaml:"explicit_cases"`
	CaseCount     int `yaml:"case_count"`
}

type ManifestField struct {
	ID            string        `yaml:"id"`
	Path          string        `yaml:"path"`
	Class         MergeClass    `yaml:"class"`
	MatrixCases   int           `yaml:"matrix_cases"`
	CrossCases    int           `yaml:"cross_cases"`
	PairSemantics PairSemantics `yaml:"pair_semantics"`
}

func (f FieldSpec) MetadataPath() string { return joinPath(f.Path) }
