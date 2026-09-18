package corpus

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Write(root string, cases []Case, manifest Manifest) error {
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	for _, item := range cases {
		dir := filepath.Join(root, item.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		metadata, err := yaml.Marshal(item.Metadata)
		if err != nil {
			return err
		}
		files := map[string][]byte{
			"case.yml": metadata, "oldbase.yml": item.BaseOld, "user.yml": item.User,
			"newbase.yml": item.BaseNew, "expected.yml": item.Expected,
		}
		for name, content := range files {
			if len(content) == 0 {
				content = []byte("{}\n")
			}
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				return err
			}
		}
	}
	content, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.yml"), content, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
