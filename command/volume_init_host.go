// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package command

import "strings"

// The example specifications for a dynamic host volume. They use invented
// names and are accepted by hostDecodeVolume.

var defaultHclHostVolumeSpec = strings.TrimSpace(`
name      = "data-a"
type      = "host"
plugin_id = "mkdir"

# Optional: the namespace of the volume. A volume with an ID updates the
# stored volume.
namespace = "default"
# id = "00000000-0000-0000-0000-000000000000"

# Optional: place the volume on one node or in one node pool.
node_pool = "default"
# node_id = "00000000-0000-0000-0000-000000000000"

# Optional: for 'wander volume create', specify a minimum and maximum
# capacity.
capacity_min = "1GiB"
capacity_max = "2GiB"

# For 'wander volume register', provide the directory that exists on the node
# and its size. 'wander volume create' sets these fields.
# host_path = "/srv/data-a"
# capacity  = "1GiB"

# Required: one or more capabilities of the volume.
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

# Optional: restrict the nodes that may hold the volume.
constraint {
  attribute = "${attr.kernel.name}"
  operator  = "="
  value     = "linux"
}

# Optional: provide a map of keys to string values expected by the plugin.
parameters {
  example_key = "example_value"
}
`)

var defaultJsonHostVolumeSpec = strings.TrimSpace(`
{
  "name": "data-a",
  "type": "host",
  "plugin_id": "mkdir",
  "namespace": "default",
  "node_pool": "default",
  "capacity_min": "1GiB",
  "capacity_max": "2GiB",
  "capability": [
    {
      "access_mode": "single-node-writer",
      "attachment_mode": "file-system"
    }
  ],
  "constraint": [
    {
      "attribute": "${attr.kernel.name}",
      "operator": "=",
      "value": "linux"
    }
  ],
  "parameters": {
    "example_key": "example_value"
  }
}
`)
