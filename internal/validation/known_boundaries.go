package validation

// knownAlgorithmBoundary records, per fixture field path, how many cases the
// two-delta SMD intent engine does not yet reproduce. The fixture corpus is
// generated from user intent only and is never shaped around the algorithm, so
// these counts are the observable boundary of the current implementation.
//
// The map is a two-way ratchet:
//
//   - a field that exceeds its budget fails the regression gate (new gap);
//   - a field that is below its budget also fails, so fixing a boundary forces
//     the budget to be tightened in the same change.
//
// Every entry groups cases by one root cause:
//
//   - free-form maps (driver_opts, ipam.options, logging.options, storage_opt,
//     ulimits, x-casaos localized text, deploy.labels, aux_addresses, device
//     options): the SMD schema stores them as untyped atomic maps, so a change
//     to one key replaces the whole mapping instead of merging key by key.
//   - deploy generic_resources and ports host_ip/protocol: the changed value is
//     part of the list identity key, so a modification is observed as an add
//     plus a remove rather than a modification of one logical item.
//   - deploy devices capabilities/device_ids: nested set attributes are typed
//     as whole values inside the device item.
//   - volume bind/tmpfs/volume attributes: nested option mappings are typed as
//     untyped atomic values inside the volume item.
//   - depends_on entries: the long-form entry mapping is typed as an untyped
//     atomic value, so condition/required cannot be merged attribute-wise.
var knownAlgorithmBoundary = map[string]int{
	"networks.default.driver_opts":                                 66,
	"networks.default.ipam.config":                                 7,
	"networks.default.ipam.options":                                66,
	"secrets.default.driver_opts":                                  66,
	"services.app.build.ulimits":                                   66,
	"services.app.depends_on":                                      1,
	"services.app.deploy.labels":                                   132,
	"services.app.logging.options":                                 66,
	"services.app.storage_opt":                                     66,
	"services.app.ulimits":                                         66,
	"volumes.default.driver_opts":                                  66,
	"x-casaos.description":                                         66,
	"x-casaos.image":                                               66,
	"x-casaos.tagline":                                             66,
	"x-casaos.title":                                               66,
	"x-casaos.tips.before_install":                                 66,
	"networks.default.ipam.config[].aux_addresses":                 60,
	"services.app.deploy.resources.limits.devices[].options":       60,
	"services.app.deploy.resources.reservations.devices[].options": 60,
	"services.app.deploy.resources.limits.devices":                 7,
	"services.app.deploy.resources.reservations.devices":           6,
	"services.app.deploy.resources.limits.generic_resources":       1,
	"services.app.deploy.resources.reservations.generic_resources": 1,
	"services.app.deploy.resources.limits.generic_resources[].discrete_resource_spec.kind":       3,
	"services.app.deploy.resources.reservations.generic_resources[].discrete_resource_spec.kind": 3,
	"services.app.ports":                           1,
	"services.app.ports[].host_ip":                 3,
	"services.app.ports[].protocol":                1,
	"services.app.volumes[].bind.create_host_path": 1,
	"services.app.volumes[].bind.propagation":      1,
	"services.app.volumes[].bind.selinux":          1,
	"services.app.volumes[].tmpfs.mode":            1,
	"services.app.volumes[].tmpfs.size":            1,
	"services.app.volumes[].volume.nocopy":         1,
}
