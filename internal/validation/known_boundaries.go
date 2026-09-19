package validation

// knownAlgorithmBoundary is a two-way ratchet over observable mismatches after
// excluding knownUnsupportedIntent. Both increases and decreases fail the gate.
// Expectations remain independently generated; fixes tighten these budgets.
//
// Remaining categories:
//   - mutable/positional list identity (ports, generic resources, IPAM);
//   - restoring a deleted named-map owner (depends_on);
//   - nested map keys whose override resets an entire keyed owner, or whose
//     upstream deletes that owner (device options and IPAM aux_addresses).
//
// The last category needs an explicit owner-versus-child deletion policy:
// blindly shrinking all keyed-item removals would break actual item deletions.
var knownAlgorithmBoundary = map[string]int{
	"networks.default.ipam.config":                                                               1,
	"networks.default.ipam.config[].aux_addresses":                                               14,
	"services.app.depends_on":                                                                    1,
	"services.app.deploy.resources.reservations.devices[].options":                               14,
	"services.app.deploy.resources.reservations.generic_resources":                               1,
	"services.app.deploy.resources.reservations.generic_resources[].discrete_resource_spec.kind": 3,
	"services.app.ports":                                                                         1,
	"services.app.ports[].host_ip":                                                               3,
	"services.app.ports[].protocol":                                                              1,
}
