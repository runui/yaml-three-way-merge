package corpus

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGeneratePreservesCorpusCardinality(t *testing.T) {
	cases, manifest, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 16132 || manifest.CaseCount != 16132 {
		t.Fatalf("generated %d cases, manifest says %d", len(cases), manifest.CaseCount)
	}
}

func TestOrderedListPairsUseWholeListSemantics(t *testing.T) {
	for _, field := range ArrayFields() {
		if field.Class == ClassOrderedList && field.PairSemantics != PairAsWholeList {
			t.Fatalf("%s has pair semantics %q", field.ID, field.PairSemantics)
		}
	}
}

func TestBlkioValuesExerciseModification(t *testing.T) {
	for _, field := range ArrayFields() {
		if field.ID != "blkio.weight-device" && field.ID != "blkio.device-read-bps" && field.ID != "blkio.device-read-iops" && field.ID != "blkio.device-write-bps" && field.ID != "blkio.device-write-iops" {
			continue
		}
		if field.Values[0] == nil || field.Values[1] == nil || field.Values[2] == nil {
			t.Fatalf("%s has incomplete values", field.ID)
		}
		for index := range field.Values {
			left, _ := yaml.Marshal(field.Values[index])
			for other := index + 1; other < len(field.Values); other++ {
				right, _ := yaml.Marshal(field.Values[other])
				if bytes.Equal(left, right) {
					t.Fatalf("%s value %d equals value %d", field.ID, index, other)
				}
			}
		}
	}
}
