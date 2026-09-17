package smdmerge

import "sigs.k8s.io/structured-merge-diff/v7/typed"

const schemaYAML = typed.YAMLObject(`types:
- name: Compose
  map:
    fields:
    - name: services
      type:
        namedType: Services
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
    - name: environment
      type:
        namedType: ScalarMap
    - name: labels
      type:
        namedType: ScalarMap
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
    - name: depends_on
      type:
        namedType: NamedMap
    - name: networks
      type:
        namedType: NamedMap
    - name: env_file
      type:
        namedType: StringSet
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
    - name: command
      type:
        namedType: AtomicValue
    - name: entrypoint
      type:
        namedType: AtomicValue
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: ScalarMap
  map:
    elementType:
      scalar: untyped
    elementRelationship: separable
- name: NamedMap
  map:
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
    - target
- name: FileReference
  map:
    fields:
    - name: target
      type:
        scalar: string
    elementType:
      namedType: UntypedAtomic
    elementRelationship: separable
- name: StringSet
  list:
    elementType:
      scalar: string
    elementRelationship: associative
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
- name: UntypedDeduced
  scalar: untyped
  list:
    elementType:
      namedType: UntypedAtomic
    elementRelationship: atomic
  map:
    elementType:
      namedType: UntypedDeduced
    elementRelationship: separable
`)
