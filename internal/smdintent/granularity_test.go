package smdintent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Decode the result directly: semantic comparison uses the same normalizer as
// compilation and cannot independently catch discarded nested attributes.
func TestGranularIntentPreservesIndependentAttributes(t *testing.T) {
	cases := []struct {
		name, old, user, upstream, expected string
	}{
		{
			name:     "free map keys and unrelated logging property",
			old:      `services: {app: {logging: {driver: json-file, options: {max-size: 10m, max-file: "3"}}}}`,
			user:     `services: {app: {logging: {options: {max-size: 20m}}}}`,
			upstream: `services: {app: {logging: {driver: local, options: {max-size: 10m, max-file: "5"}}}}`,
			expected: `services: {app: {logging: {driver: local, options: {max-size: 20m, max-file: "5"}}}}`,
		},
		{
			name:     "deploy labels list and map syntax",
			old:      `services: {app: {deploy: {labels: [team=old, region=old]}}}`,
			user:     `services: {app: {deploy: {labels: {team: user}}}}`,
			upstream: `services: {app: {deploy: {labels: [team=old, region=new]}}}`,
			expected: `services: {app: {deploy: {labels: {team: user, region: new}}}}`,
		},
		{
			name:     "volume nested options remain observable",
			old:      `services: {app: {volumes: [{type: bind, source: /host, target: /data, bind: {propagation: rprivate, create_host_path: true}}]}}`,
			user:     `services: {app: {volumes: [{type: bind, source: /host, target: /data, bind: {propagation: rshared}}]}}`,
			upstream: `services: {app: {volumes: [{type: bind, source: /host, target: /data, bind: {propagation: rprivate, create_host_path: false}}]}}`,
			expected: `services: {app: {volumes: [{type: bind, source: /host, target: /data, read_only: false, consistency: "", bind: {propagation: rshared, create_host_path: false}}]}}`,
		},
		{
			name:     "restore outer owner of nested keyed write",
			old:      `services: {app: {deploy: {resources: {reservations: {devices: [{driver: nvidia, capabilities: [gpu-a], device_ids: [card-a]}]}}}}}`,
			user:     `services: {app: {deploy: {resources: {reservations: {devices: [{driver: nvidia, capabilities: [tpu-a]}]}}}}}`,
			upstream: `services: {app: {image: busybox}}`,
			expected: `services: {app: {image: busybox, deploy: {resources: {reservations: {devices: [{driver: nvidia, capabilities: [tpu-a], device_ids: [card-a]}]}}}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Merge([]byte(tc.old), []byte(tc.user), []byte(tc.upstream))
			require.NoError(t, err)
			assert.True(t, result.Report.UserReplayValid)
			assert.True(t, result.Report.UpstreamReplayValid)
			var actual, expected map[string]any
			require.NoError(t, yaml.Unmarshal(result.YAML, &actual))
			require.NoError(t, yaml.Unmarshal([]byte(tc.expected), &expected))
			assert.Equal(t, expected, actual)
		})
	}
}
