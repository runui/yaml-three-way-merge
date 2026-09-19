package smdmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNamedMapForms(t *testing.T) {
	document, err := normalizeYAML([]byte(`services:
  app:
    annotations: [annotation=value]
    extra_hosts: [host=127.0.0.1]
    sysctls: [net.core.value=1]
    build:
      labels: [build.label=value]
      additional_contexts: [context=./context]
      extra_hosts: [build-host=127.0.0.2]
volumes:
  default:
    labels: [volume.label=value]
`))
	require.NoError(t, err)

	service := document["services"].(map[string]any)["app"].(map[string]any)
	assert.Equal(t, "value", service["annotations"].(map[string]any)["annotation"])
	assert.Equal(t, "127.0.0.1", service["extra_hosts"].(map[string]any)["host"])
	assert.Equal(t, "1", service["sysctls"].(map[string]any)["net.core.value"])
	build := service["build"].(map[string]any)
	assert.Equal(t, "value", build["labels"].(map[string]any)["build.label"])
	assert.Equal(t, "./context", build["additional_contexts"].(map[string]any)["context"])
	assert.Equal(t, "127.0.0.2", build["extra_hosts"].(map[string]any)["build-host"])
	volume := document["volumes"].(map[string]any)["default"].(map[string]any)
	assert.Equal(t, "value", volume["labels"].(map[string]any)["volume.label"])
}
