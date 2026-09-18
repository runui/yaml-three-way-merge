package smdmerge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeDeterministicFieldSemantics(t *testing.T) {
	tests := []struct {
		name     string
		baseOld  string
		override string
		baseNew  string
		expected string
	}{
		{
			name:     "named map keeps independent remote keys",
			baseOld:  "services:\n  app:\n    environment: [A=old]\n",
			override: "services:\n  app:\n    environment: [A=user]\n",
			baseNew:  "services:\n  app:\n    environment: [A=remote, B=remote]\n",
			expected: "services:\n  app:\n    environment:\n      A: user\n      B: remote\n",
		},
		{
			name:     "ordered env file is replaced as a whole",
			baseOld:  "services:\n  app:\n    env_file: [base.env, shared.env]\n",
			override: "services:\n  app:\n    env_file: [user.env, shared.env]\n",
			baseNew:  "services:\n  app:\n    env_file: [remote.env, shared.env]\n",
			expected: "services:\n  app:\n    env_file: [user.env, shared.env]\n",
		},
		{
			name:     "build named maps are separable",
			baseOld:  "services:\n  app:\n    build:\n      args: [A=old]\n",
			override: "services:\n  app:\n    build:\n      args: [A=user]\n",
			baseNew:  "services:\n  app:\n    build:\n      args: [A=remote, B=remote]\n",
			expected: "services:\n  app:\n    build:\n      args:\n        A: user\n        B: remote\n",
		},
		{
			name:     "target keyed build secrets retain remote additions",
			baseOld:  "services:\n  app:\n    build:\n      secrets:\n        - source: old\n          target: /run/a\n",
			override: "services:\n  app:\n    build:\n      secrets:\n        - source: user\n          target: /run/a\n",
			baseNew:  "services:\n  app:\n    build:\n      secrets:\n        - source: remote\n          target: /run/a\n        - source: added\n          target: /run/b\n",
			expected: "services:\n  app:\n    build:\n      secrets:\n        - source: user\n          target: /run/a\n        - source: added\n          target: /run/b\n",
		},
		{
			name:     "path keyed blkio items retain remote additions",
			baseOld:  "services:\n  app:\n    blkio_config:\n      weight_device: [{path: /dev/a, weight: 100}]\n",
			override: "services:\n  app:\n    blkio_config:\n      weight_device: [{path: /dev/a, weight: 200}]\n",
			baseNew:  "services:\n  app:\n    blkio_config:\n      weight_device: [{path: /dev/a, weight: 300}, {path: /dev/b, weight: 400}]\n",
			expected: "services:\n  app:\n    blkio_config:\n      weight_device: [{path: /dev/a, weight: 200}, {path: /dev/b, weight: 400}]\n",
		},
		{
			name:     "generic resource kind is promoted as identity",
			baseOld:  "services:\n  app:\n    deploy:\n      resources:\n        limits:\n          generic_resources: [{discrete_resource_spec: {kind: GPU, value: 1}}]\n",
			override: "services:\n  app:\n    deploy:\n      resources:\n        limits:\n          generic_resources: [{discrete_resource_spec: {kind: GPU, value: 2}}]\n",
			baseNew:  "services:\n  app:\n    deploy:\n      resources:\n        limits:\n          generic_resources: [{discrete_resource_spec: {kind: GPU, value: 3}}, {discrete_resource_spec: {kind: TPU, value: 1}}]\n",
			expected: "services:\n  app:\n    deploy:\n      resources:\n        limits:\n          generic_resources: [{discrete_resource_spec: {kind: GPU, value: 2}}, {discrete_resource_spec: {kind: TPU, value: 1}}]\n",
		},
		{
			name:     "top level resource labels are separable",
			baseOld:  "networks:\n  default:\n    labels: [A=old]\n",
			override: "networks:\n  default:\n    labels: [A=user]\n",
			baseNew:  "networks:\n  default:\n    labels: [A=remote, B=remote]\n",
			expected: "networks:\n  default:\n    labels:\n      A: user\n      B: remote\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := Merge([]byte(tt.baseOld), []byte(tt.override), []byte(tt.baseNew))
			require.NoError(t, err)
			equal, err := Equivalent(actual, []byte(tt.expected))
			require.NoError(t, err)
			assert.True(t, equal, "actual:\n%s\nexpected:\n%s", actual, tt.expected)
			assert.NotContains(t, string(actual), "__merge_id")
		})
	}
}

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
