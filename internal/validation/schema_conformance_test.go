package validation

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	composeschema "github.com/compose-spec/compose-go/schema"
	"github.com/runui/yaml-three-way-merge/internal/corpus"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestFixtureDocumentsConformToComposeSchema validates every generated
// oldbase/newbase/expected document against the pinned Compose schema. A
// fixture that is not valid Compose cannot occur in a real repository, so it
// would test merge semantics over inputs users can never author. The user
// override (user.yml) is written with `!reset` tags and is validated by the
// overlay-specific tests instead.
func TestFixtureDocumentsConformToComposeSchema(t *testing.T) {
	cases, _, err := corpus.Load(filepath.Join("..", "..", "fixtures"))
	require.NoError(t, err)
	for _, item := range cases {
		for name, content := range map[string][]byte{
			"oldbase.yml":  item.BaseOld,
			"newbase.yml":  item.BaseNew,
			"expected.yml": item.Expected,
		} {
			document := decodeMapping(t, content)
			if len(document) == 0 {
				continue
			}
			if err := composeschema.Validate(document); err != nil {
				t.Errorf("%s/%s is not valid Compose: %v", item.Metadata.ID, name, err)
			}
		}
	}
}

func decodeMapping(t *testing.T, content []byte) map[string]any {
	t.Helper()
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document map[string]any
	if err := decoder.Decode(&document); err != nil && err != io.EOF {
		t.Fatalf("decode document: %v", err)
	}
	if document == nil {
		document = map[string]any{}
	}
	return document
}
