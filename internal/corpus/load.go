package corpus

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func Load(root string) ([]Case, Manifest, error) {
	manifestContent, err := os.ReadFile(filepath.Join(root, "manifest.yml"))
	if err != nil {
		return nil, Manifest{}, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(manifestContent, &manifest); err != nil {
		return nil, Manifest{}, err
	}
	var caseDirs []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() == "case.yml" {
			caseDirs = append(caseDirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, Manifest{}, err
	}
	sort.Strings(caseDirs)
	cases := make([]Case, 0, len(caseDirs))
	for _, dir := range caseDirs {
		item, err := loadCase(root, dir)
		if err != nil {
			return nil, Manifest{}, err
		}
		cases = append(cases, item)
	}
	if len(cases) != manifest.CaseCount {
		return nil, Manifest{}, fmt.Errorf("manifest declares %d cases, found %d", manifest.CaseCount, len(cases))
	}
	return cases, manifest, nil
}

func loadCase(root, dir string) (Case, error) {
	read := func(name string) ([]byte, error) {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
		}
		return content, nil
	}
	metadataContent, err := read("case.yml")
	if err != nil {
		return Case{}, err
	}
	var metadata Metadata
	if err := yaml.Unmarshal(metadataContent, &metadata); err != nil {
		return Case{}, fmt.Errorf("%s: %w", filepath.Join(dir, "case.yml"), err)
	}
	baseOld, err := read("oldbase.yml")
	if err != nil {
		return Case{}, err
	}
	user, err := read("user.yml")
	if err != nil {
		return Case{}, err
	}
	baseNew, err := read("newbase.yml")
	if err != nil {
		return Case{}, err
	}
	expected, err := read("expected.yml")
	if err != nil {
		return Case{}, err
	}
	relative, err := filepath.Rel(root, dir)
	if err != nil {
		return Case{}, err
	}
	return Case{Metadata: metadata, Dir: relative, BaseOld: baseOld, User: user, BaseNew: baseNew, Expected: expected}, nil
}
