// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import "github.com/hashicorp/go-memdb"

const (
	TableDynamicHostVolumes           = "dynamic_host_volumes"
	TableDynamicHostVolumeClaims      = "dynamic_host_volume_claims"
	dynamicHostVolumeNodeNameIndex    = "node_name"
	dynamicHostVolumeNodeOnlyIndex    = "node_only"
	dynamicHostVolumePluginIndex      = "plugin_id"
	dynamicHostVolumeClaimVolumeIndex = "volume_id"
)

func init() {
	RegisterSchemaFactories(dynamicHostVolumesSchema, dynamicHostVolumeClaimsSchema)
}

func dynamicHostVolumeIndex(name string, unique, optional bool, fields ...string) *memdb.IndexSchema {
	indexers := make([]memdb.Indexer, len(fields))
	for index, field := range fields {
		indexers[index] = &memdb.StringFieldIndex{Field: field}
	}
	return &memdb.IndexSchema{Name: name, Unique: unique, AllowMissing: optional,
		Indexer: &memdb.CompoundIndex{Indexes: indexers}}
}

func dynamicHostVolumesSchema() *memdb.TableSchema {
	return &memdb.TableSchema{Name: TableDynamicHostVolumes, Indexes: map[string]*memdb.IndexSchema{
		indexID:     dynamicHostVolumeIndex(indexID, true, false, "Namespace", "ID"),
		indexNodeID: dynamicHostVolumeIndex(indexNodeID, false, true, "Namespace", "NodeID", "ID"),
		// The node-only index spans namespaces, so a node write finds all of
		// its volumes in one lookup. Volumes with no node are not indexed.
		dynamicHostVolumeNodeOnlyIndex: dynamicHostVolumeIndex(dynamicHostVolumeNodeOnlyIndex, false, true, "NodeID"),
		dynamicHostVolumePluginIndex:   dynamicHostVolumeIndex(dynamicHostVolumePluginIndex, false, true, "Namespace", "PluginID", "ID"),
		dynamicHostVolumeNodeNameIndex: dynamicHostVolumeIndex(dynamicHostVolumeNodeNameIndex, true, true, "Namespace", "NodeID", "Name"),
	}}
}

func dynamicHostVolumeClaimsSchema() *memdb.TableSchema {
	return &memdb.TableSchema{Name: TableDynamicHostVolumeClaims, Indexes: map[string]*memdb.IndexSchema{
		indexID:                           dynamicHostVolumeIndex(indexID, true, false, "Namespace", "ID"),
		dynamicHostVolumeClaimVolumeIndex: dynamicHostVolumeIndex(dynamicHostVolumeClaimVolumeIndex, false, false, "Namespace", "VolumeID", "ID"),
		indexJob:                          dynamicHostVolumeIndex(indexJob, false, false, "Namespace", "JobID", "ID"),
	}}
}
