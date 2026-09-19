package smdmodel

import "sigs.k8s.io/structured-merge-diff/v7/typed"

const schemaYAML = typed.YAMLObject(`types:
- name: Compose
  map:
    fields:
    - name: include
      type:
        namedType: AtomicList
    - name: services
      type:
        namedType: Services
    - name: networks
      type:
        namedType: Resources
    - name: volumes
      type:
        namedType: Resources
    - name: configs
      type:
        namedType: Resources
    - name: secrets
      type:
        namedType: Resources
    - name: x-casaos
      type:
        namedType: CasaOS
    - name: x-validation
      type:
        namedType: ValidationExtension
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Services
  map:
    elementType:
      namedType: Service
    elementRelationship: separable
- name: Service
  map:
    fields:
    - name: annotations
      type:
        namedType: ScalarMap
    - name: environment
      type:
        namedType: ScalarMap
    - name: extra_hosts
      type:
        namedType: ScalarMap
    - name: labels
      type:
        namedType: ScalarMap
    - name: sysctls
      type:
        namedType: ScalarMap
    - name: depends_on
      type:
        namedType: DependencyMap
    - name: extends
      type:
        namedType: MappingValue
    - name: logging
      type:
        namedType: Logging
    - name: storage_opt
      type:
        namedType: ScalarMap
    - name: ulimits
      type:
        namedType: ValueMap
    - name: networks
      type:
        namedType: ServiceNetworks
    - name: ports
      type:
        namedType: PortList
    - name: volumes
      type:
        namedType: VolumeList
    - name: devices
      type:
        namedType: DeviceList
    - name: configs
      type:
        namedType: FileReferenceList
    - name: secrets
      type:
        namedType: FileReferenceList
    - name: env_file
      type:
        namedType: AtomicList
    - name: cap_add
      type:
        namedType: StringSet
    - name: cap_drop
      type:
        namedType: StringSet
    - name: dns
      type:
        namedType: StringSet
    - name: dns_opt
      type:
        namedType: StringSet
    - name: dns_search
      type:
        namedType: StringSet
    - name: expose
      type:
        namedType: StringSet
    - name: profiles
      type:
        namedType: StringSet
    - name: tmpfs
      type:
        namedType: StringSet
    - name: device_cgroup_rules
      type:
        namedType: StringSet
    - name: external_links
      type:
        namedType: StringSet
    - name: group_add
      type:
        namedType: StringSet
    - name: links
      type:
        namedType: StringSet
    - name: security_opt
      type:
        namedType: StringSet
    - name: volumes_from
      type:
        namedType: StringSet
    - name: command
      type:
        namedType: AtomicValue
    - name: entrypoint
      type:
        namedType: AtomicValue
    - name: build
      type:
        namedType: Build
    - name: develop
      type:
        namedType: Develop
    - name: blkio_config
      type:
        namedType: BlkioConfig
    - name: deploy
      type:
        namedType: Deploy
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Build
  scalar: untyped
  map:
    fields:
    - name: ulimits
      type:
        namedType: ValueMap
    - name: args
      type:
        namedType: ScalarMap
    - name: labels
      type:
        namedType: ScalarMap
    - name: additional_contexts
      type:
        namedType: ScalarMap
    - name: extra_hosts
      type:
        namedType: ScalarMap
    - name: secrets
      type:
        namedType: FileReferenceList
    - name: ssh
      type:
        namedType: StringSet
    - name: cache_from
      type:
        namedType: AtomicList
    - name: cache_to
      type:
        namedType: AtomicList
    - name: tags
      type:
        namedType: AtomicList
    - name: platforms
      type:
        namedType: AtomicList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Develop
  map:
    fields:
    - name: watch
      type:
        namedType: AtomicList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: BlkioConfig
  map:
    fields:
    - name: weight_device
      type:
        namedType: PathList
    - name: device_read_bps
      type:
        namedType: PathList
    - name: device_read_iops
      type:
        namedType: PathList
    - name: device_write_bps
      type:
        namedType: PathList
    - name: device_write_iops
      type:
        namedType: PathList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Deploy
  map:
    fields:
    - name: labels
      type:
        namedType: ScalarMap
    - name: resources
      type:
        namedType: DeployResources
    - name: placement
      type:
        namedType: Placement
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: DeployResources
  map:
    fields:
    - name: limits
      type:
        namedType: ResourceLimits
    - name: reservations
      type:
        namedType: ResourceLimits
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ResourceLimits
  map:
    fields:
    - name: generic_resources
      type:
        namedType: GenericResourceList
    - name: devices
      type:
        namedType: DeployDeviceList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: DeployDeviceList
  list:
    elementType:
      namedType: DeployDevice
    elementRelationship: associative
    keys:
    - __merge_id
- name: DeployDevice
  map:
    fields:
    - name: options
      type:
        namedType: ScalarMap
    - name: __merge_id
      type:
        scalar: string
    - name: driver
      type:
        scalar: string
    - name: capabilities
      type:
        namedType: StringSet
    - name: device_ids
      type:
        namedType: StringSet
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Placement
  map:
    fields:
    - name: constraints
      type:
        namedType: AtomicList
    - name: preferences
      type:
        namedType: AtomicList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Resources
  map:
    elementType:
      namedType: ResourceDefinition
    elementRelationship: separable
- name: ResourceDefinition
  map:
    fields:
    - name: driver_opts
      type:
        namedType: ScalarMap
    - name: labels
      type:
        namedType: ScalarMap
    - name: ipam
      type:
        namedType: Ipam
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Ipam
  map:
    fields:
    - name: options
      type:
        namedType: ScalarMap
    - name: config
      type:
        namedType: IpamConfigList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: IpamConfigList
  list:
    elementType:
      namedType: IpamConfig
    elementRelationship: associative
    keys:
    - __merge_id
- name: IpamConfig
  map:
    fields:
    - name: aux_addresses
      type:
        namedType: ScalarMap
    - name: __merge_id
      type:
        scalar: string
    - name: subnet
      type:
        scalar: string
    - name: ip_range
      type:
        scalar: string
    - name: gateway
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: CasaOS
  map:
    fields:
    - name: description
      type:
        namedType: ScalarMap
    - name: image
      type:
        namedType: ScalarMap
    - name: tagline
      type:
        namedType: ScalarMap
    - name: title
      type:
        namedType: ScalarMap
    - name: tips
      type:
        namedType: CasaOSTips
    - name: architectures
      type:
        namedType: StringSet
    - name: screenshot_link
      type:
        namedType: AtomicList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ValidationExtension
  map:
    fields:
    - name: items
      type:
        namedType: AtomicList
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ScalarMap
  map:
    elementType:
      scalar: untyped
    elementRelationship: separable
- name: CasaOSTips
  map:
    fields:
    - name: before_install
      type:
        namedType: ScalarMap
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: Logging
  map:
    fields:
    - name: options
      type:
        namedType: ScalarMap
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: DependencyMap
  map:
    elementType:
      namedType: MappingValue
    elementRelationship: separable
- name: MappingValue
  scalar: untyped
  map:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ValueMap
  map:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ServiceNetworks
  map:
    elementType:
      namedType: ServiceNetwork
    elementRelationship: separable
- name: ServiceNetwork
  map:
    fields:
    - name: aliases
      type:
        namedType: StringSet
    - name: link_local_ips
      type:
        namedType: StringSet
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: PortList
  list:
    elementType:
      namedType: Port
    elementRelationship: associative
    keys:
    - __merge_id
    - host_ip
    - target
    - protocol
- name: Port
  map:
    fields:
    - name: __merge_id
      type:
        scalar: string
    - name: host_ip
      type:
        scalar: string
    - name: target
      type:
        scalar: numeric
    - name: published
      type:
        scalar: string
    - name: protocol
      type:
        scalar: string
    - name: mode
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: VolumeList
  list:
    elementType:
      namedType: Volume
    elementRelationship: associative
    keys:
    - target
- name: Volume
  map:
    fields:
    - name: bind
      type:
        namedType: MappingValue
    - name: tmpfs
      type:
        namedType: MappingValue
    - name: volume
      type:
        namedType: MappingValue
    - name: target
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: DeviceList
  list:
    elementType:
      namedType: Device
    elementRelationship: associative
    keys:
    - target
- name: Device
  map:
    fields:
    - name: target
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: FileReferenceList
  list:
    elementType:
      namedType: FileReference
    elementRelationship: associative
    keys:
    - __merge_id
- name: FileReference
  map:
    fields:
    - name: __merge_id
      type:
        scalar: string
    - name: target
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: PathList
  list:
    elementType:
      namedType: PathItem
    elementRelationship: associative
    keys:
    - path
- name: PathItem
  map:
    fields:
    - name: path
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: GenericResourceList
  list:
    elementType:
      namedType: GenericResource
    elementRelationship: associative
    keys:
    - __merge_id
- name: GenericResource
  map:
    fields:
    - name: __merge_id
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: StringSet
  list:
    elementType:
      namedType: StringSetItem
    elementRelationship: associative
    keys:
    - __merge_id
- name: StringSetItem
  map:
    fields:
    - name: __merge_id
      type:
        scalar: string
    - name: __value
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: AtomicList
  list:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: atomic
- name: AtomicValue
  scalar: untyped
  list:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: atomic
- name: UntypedAtomic
  scalar: untyped
  list:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: atomic
  map:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: atomic
`)
