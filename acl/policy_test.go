// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	ci.Parallel(t)

	type tcase struct {
		Raw    string
		ErrStr string
		Expect *Policy
	}
	tcases := []tcase{
		{
			`
			namespace "default" {
				policy = "read"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityHostVolumeRead,
						},
					},
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "read"
			}
			namespace "other" {
				policy = "write"
			}
			namespace "secret" {
				capabilities = ["deny", "read-logs"]
			}
			namespace "apps" {
				variables {
					path "jobs/write-does-not-imply-read-or-delete" {
						capabilities = ["write"]
					}
					path "project/read-implies-list" {
						capabilities = ["read"]
					}
					path "project/explicit" {
						capabilities = ["read", "list", "destroy"]
					}
				}
			}
			namespace "autoscaler" {
				policy = "scale"
			}
			host_volume "production-tls-*" {
				capabilities = ["mount-readonly"]
			}
			host_volume "staging-tls-*" {
				policy = "write"
			}
			node_pool "prod" {
				capabilities = ["read"]
			}
			node_pool "dev" {
				policy = "write"
			}
			agent {
				policy = "read"
			}
			node {
				policy = "write"
			}
			operator {
				policy = "deny"
			}
			quota {
				policy = "read"
			}
			plugin {
				policy = "read"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityHostVolumeRead,
						},
					},
					{
						Name:   "other",
						Policy: PolicyWrite,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityHostVolumeRead,
							NamespaceCapabilityScaleJob,
							NamespaceCapabilitySubmitJob,
							NamespaceCapabilityDispatchJob,
							NamespaceCapabilityReadLogs,
							NamespaceCapabilityReadFS,
							NamespaceCapabilityAllocExec,
							NamespaceCapabilityAllocLifecycle,
							NamespaceCapabilityCSIMountVolume,
							NamespaceCapabilityCSIWriteVolume,
							NamespaceCapabilitySubmitRecommendation,
							NamespaceCapabilityHostVolumeWrite,
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRegister,
							NamespaceCapabilityHostVolumeDelete,
						},
					},
					{
						Name: "secret",
						Capabilities: []string{
							NamespaceCapabilityDeny,
							NamespaceCapabilityReadLogs,
						},
					},
					{
						Name: "apps",
						Variables: &VariablesPolicy{
							Paths: []*VariablesPathPolicy{
								{
									PathSpec:     "jobs/write-does-not-imply-read-or-delete",
									Capabilities: []string{VariablesCapabilityWrite},
								},
								{
									PathSpec: "project/read-implies-list",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
									},
								},
								{
									PathSpec: "project/explicit",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
										VariablesCapabilityDestroy,
									},
								},
							},
						},
					},
					{
						Name:   "autoscaler",
						Policy: PolicyScale,
						Capabilities: []string{
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityScaleJob,
						},
					},
				},
				HostVolumes: []*HostVolumePolicy{
					{
						Name:         "production-tls-*",
						Capabilities: []string{"mount-readonly"},
					},
					{
						Name:   "staging-tls-*",
						Policy: "write",
						Capabilities: []string{
							"mount-readonly",
							"mount-readwrite",
						},
					},
				},
				NodePools: []*NodePoolPolicy{
					{
						Name:         "prod",
						Capabilities: []string{"read"},
					},
					{
						Name:         "dev",
						Policy:       "write",
						Capabilities: []string{"delete", "read", "write"},
					},
				},
				Agent: &AgentPolicy{
					Policy: PolicyRead,
				},
				Node: &NodePolicy{
					Policy: PolicyWrite,
				},
				Operator: &OperatorPolicy{
					Policy: PolicyDeny,
				},
				Quota: &QuotaPolicy{
					Policy: PolicyRead,
				},
				Plugin: &PluginPolicy{
					Policy: PolicyRead,
				},
			},
		},
		{
			`
			{
				"namespace": [
					{
						"default": {
							"policy": "read"
						},
					},
					{
						"other": {
							"policy": "write"
						},
					},
					{
						"secret": {
							"capabilities": [
								"deny",
								"read-logs"
							]
						}
					},
					{
						"apps": {
							"variables": [
								{
									"path": [
										{
											"jobs/write-does-not-imply-read-or-delete": {
												"capabilities": ["write"],
											},
										},
										{
											"project/read-implies-list": {
												"capabilities": ["read"],
											},
										},
										{
											"project/explicit": {
												"capabilities": ["read", "list", "destroy"],
											},
										},
									],
								},
							],
						},
					},
					{
						"autoscaler": {
							"policy": "scale"
						},
					},
				],
				"host_volume": [
					{
						"production-tls-*": {
							"capabilities": ["mount-readonly"]
						}
					},
					{
						"staging-tls-*": {
							"policy": "write"
						}
					}
				],
				"node_pool": [
					{
						"prod": {
							"capabilities": ["read"]
						}
					},
					{
						"dev": {
							"policy": "write"
						}
					}
				],
				"agent": {
					"policy": "read"
				},
				"node": {
					"policy": "write"
				},
				"operator": {
					"policy": "deny"
				},
				"quota": {
					"policy": "read"
				},
				"plugin": {
					"policy": "read"
				}
			}`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityHostVolumeRead,
						},
					},
					{
						Name:   "other",
						Policy: PolicyWrite,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityHostVolumeRead,
							NamespaceCapabilityScaleJob,
							NamespaceCapabilitySubmitJob,
							NamespaceCapabilityDispatchJob,
							NamespaceCapabilityReadLogs,
							NamespaceCapabilityReadFS,
							NamespaceCapabilityAllocExec,
							NamespaceCapabilityAllocLifecycle,
							NamespaceCapabilityCSIMountVolume,
							NamespaceCapabilityCSIWriteVolume,
							NamespaceCapabilitySubmitRecommendation,
							NamespaceCapabilityHostVolumeWrite,
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRegister,
							NamespaceCapabilityHostVolumeDelete,
						},
					},
					{
						Name: "secret",
						Capabilities: []string{
							NamespaceCapabilityDeny,
							NamespaceCapabilityReadLogs,
						},
					},
					{
						Name: "apps",
						Variables: &VariablesPolicy{
							Paths: []*VariablesPathPolicy{
								{
									PathSpec:     "jobs/write-does-not-imply-read-or-delete",
									Capabilities: []string{VariablesCapabilityWrite},
								},
								{
									PathSpec: "project/read-implies-list",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
									},
								},
								{
									PathSpec: "project/explicit",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
										VariablesCapabilityDestroy,
									},
								},
							},
						},
					},
					{
						Name:   "autoscaler",
						Policy: PolicyScale,
						Capabilities: []string{
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityScaleJob,
						},
					},
				},
				HostVolumes: []*HostVolumePolicy{
					{
						Name:         "production-tls-*",
						Capabilities: []string{"mount-readonly"},
					},
					{
						Name:   "staging-tls-*",
						Policy: "write",
						Capabilities: []string{
							"mount-readonly",
							"mount-readwrite",
						},
					},
				},
				NodePools: []*NodePoolPolicy{
					{
						Name:         "prod",
						Capabilities: []string{"read"},
					},
					{
						Name:         "dev",
						Policy:       "write",
						Capabilities: []string{"delete", "read", "write"},
					},
				},
				Agent: &AgentPolicy{
					Policy: PolicyRead,
				},
				Node: &NodePolicy{
					Policy: PolicyWrite,
				},
				Operator: &OperatorPolicy{
					Policy: PolicyDeny,
				},
				Quota: &QuotaPolicy{
					Policy: PolicyRead,
				},
				Plugin: &PluginPolicy{
					Policy: PolicyRead,
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "foo"
			}
			`,
			"Invalid namespace policy",
			nil,
		},
		{
			`
			namespace {
				policy = "read"
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			{
				"namespace": [
					{
						"": {
							"policy": "read"
						}
					}
				]
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			namespace "dev" {
			  variables "*" {
			      capabilities = ["read", "write"]
			  }
			}
			`,
			"Invalid variable policy: no variable paths in namespace dev",
			nil,
		},
		{
			`
			namespace "dev" {
				policy = "read"

				variables {
					path {}
					path "nomad/jobs/example" {
						capabilities = ["read"]
					}
				}
			}
			`,
			"Invalid missing variable path in namespace",
			nil,
		},
		{
			`
			{
				"namespace": [
					{
						"dev": {
							"policy": "read",
							"variables": [
								{
									"paths": [
										{
											"": {
												"capabilities": ["read"]
											}
										}
									]
								]
							]
						}
					}
				]
			}
			`,
			"no variable paths in namespace dev",
			nil,
		},
		{
			`
			namespace "default" {
				capabilities = ["deny", "foo"]
			}
			`,
			"Invalid namespace capability",
			nil,
		},
		{
			`namespace {}`,
			"invalid acl policy",
			nil,
		},
		{
			`
			agent {
				policy = "foo"
			}
			`,
			"Invalid agent policy",
			nil,
		},
		{
			`
			node {
				policy = "foo"
			}
			`,
			"Invalid node policy",
			nil,
		},
		{
			`
			operator {
				policy = "foo"
			}
			`,
			"Invalid operator policy",
			nil,
		},
		{
			`
			quota {
				policy = "foo"
			}
			`,
			"Invalid quota policy",
			nil,
		},
		{
			`
			{
				"Name": "my-policy",
				"Description": "This is a great policy",
				"Rules": "anything"
			}
			`,
			"Invalid policy",
			nil,
		},
		{
			`
			namespace "has a space"{
				policy = "read"
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			namespace "default" {
				capabilities = ["sentinel-override"]
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: "",
						Capabilities: []string{
							NamespaceCapabilitySentinelOverride,
						},
					},
				},
			},
		},
		{
			`
			node_pool "pool-read-only" {
				policy = "read"
			}

			node_pool "pool-read-write" {
				policy = "write"
			}

			node_pool "pool-read-upsert" {
				policy = "read"
				capabilities = ["write"]
			}

			node_pool "pool-multiple-capabilities" {
				policy = "read"
				capabilities = ["write", "delete"]
			}

			node_pool "pool-deny-policy" {
				policy = "deny"
				capabilities = ["write"]
			}

			node_pool "pool-deny-capability" {
				capabilities = ["deny", "read"]
			}

			node_pool "pool-*" {
				policy = "read"
			}
			`,
			"",
			&Policy{
				NodePools: []*NodePoolPolicy{
					{
						Name:   "pool-read-only",
						Policy: PolicyRead,
						Capabilities: []string{
							NodePoolCapabilityRead,
						},
					},
					{
						Name:   "pool-read-write",
						Policy: PolicyWrite,
						Capabilities: []string{
							NodePoolCapabilityDelete,
							NodePoolCapabilityRead,
							NodePoolCapabilityWrite,
						},
					},
					{
						Name:   "pool-read-upsert",
						Policy: PolicyRead,
						Capabilities: []string{
							NodePoolCapabilityWrite,
							NodePoolCapabilityRead,
						},
					},
					{
						Name:   "pool-multiple-capabilities",
						Policy: PolicyRead,
						Capabilities: []string{
							NodePoolCapabilityWrite,
							NodePoolCapabilityDelete,
							NodePoolCapabilityRead,
						},
					},
					{
						Name:   "pool-deny-policy",
						Policy: PolicyDeny,
						Capabilities: []string{
							NodePoolCapabilityWrite,
							NodePoolCapabilityDeny,
						},
					},
					{
						Name:   "pool-deny-capability",
						Policy: "",
						Capabilities: []string{
							NodePoolCapabilityDeny,
							NodePoolCapabilityRead,
						},
					},
					{
						Name:   "pool-*",
						Policy: PolicyRead,
						Capabilities: []string{
							NodePoolCapabilityRead,
						},
					},
				},
			},
		},
		{
			`
			node_pool "" {
			}
			`,
			"Invalid node pool name",
			nil,
		},
		{
			`
			node_pool "pool%" {
			}
			`,
			"Invalid node pool name",
			nil,
		},
		{
			`
			node_pool "my-pool" {
				capabilities = ["read", "invalid"]
			}
			`,
			"Invalid node pool capability",
			nil,
		},
		{
			`
			node_pool {
				policy = "read"
			}
			`,
			"Invalid node pool name",
			nil,
		},
		{
			`
			{
				"node_pool": [
					{
						"": {
							"policy": "read"
						}
					}
				]
			}
			`,
			"Invalid node pool name",
			nil,
		},
		{
			`
			host_volume "production-tls-*" {
				capabilities = ["mount-readonly"]
			}
			`,
			"",
			&Policy{
				HostVolumes: []*HostVolumePolicy{
					{
						Name:   "production-tls-*",
						Policy: "",
						Capabilities: []string{
							HostVolumeCapabilityMountReadOnly,
						},
					},
				},
			},
		},
		{
			`
			host_volume "production-tls-*" {
				capabilities = ["mount-readwrite"]
			}
			`,
			"",
			&Policy{
				HostVolumes: []*HostVolumePolicy{
					{
						Name:   "production-tls-*",
						Policy: "",
						Capabilities: []string{
							HostVolumeCapabilityMountReadWrite,
						},
					},
				},
			},
		},
		{
			`
			host_volume "volume has a space" {
				capabilities = ["mount-readwrite"]
			}
			`,
			"Invalid host volume name",
			nil,
		},
		{
			`
			host_volume {
				policy = "read"
			}
			`,
			"Invalid host volume name",
			nil,
		},
		{
			`
			{
				"host_volume": [
					{
						"": {
							"policy": "read"
						}
					}
				]
			}
			`,
			"Invalid host volume name",
			nil,
		},
		{
			`
			plugin {
				policy = "list"
			}
			`,
			"",
			&Policy{
				Plugin: &PluginPolicy{
					Policy: PolicyList,
				},
			},
		},
		{
			`
			plugin {
				policy = "reader"
			}
			`,
			"Invalid plugin policy",
			nil,
		},
	}

	for idx, tc := range tcases {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			p, err := Parse(tc.Raw)
			if err != nil {
				if tc.ErrStr == "" {
					t.Fatalf("Unexpected err: %v", err)
				}
				if !strings.Contains(err.Error(), tc.ErrStr) {
					t.Fatalf("Unexpected err: %v", err)
				}
				return
			}
			if err == nil && tc.ErrStr != "" {
				t.Fatalf("Missing expected err")
			}
			tc.Expect.Raw = tc.Raw
			assert.EqualValues(t, tc.Expect, p)
		})
	}
}

func TestParse_BadInput(t *testing.T) {
	ci.Parallel(t)

	inputs := []string{
		`namespace "\500" {}`,
	}

	for i, c := range inputs {
		t.Run(fmt.Sprintf("%d: %v", i, c), func(t *testing.T) {
			_, err := Parse(c)
			assert.Error(t, err)
		})
	}
}

var hostVolumeNamespaceCapabilities = []string{
	NamespaceCapabilityHostVolumeCreate,
	NamespaceCapabilityHostVolumeRegister,
	NamespaceCapabilityHostVolumeRead,
	NamespaceCapabilityHostVolumeWrite,
	NamespaceCapabilityHostVolumeDelete,
}

func TestParse_HostVolumeNamespaceCapabilities(t *testing.T) {
	ci.Parallel(t)

	all := hostVolumeNamespaceCapabilities
	hvOnly := func(caps []string) []string {
		var out []string
		for _, c := range caps {
			if slices.Contains(all, c) {
				out = append(out, c)
			}
		}
		slices.Sort(out)
		return out
	}
	sorted := func(caps ...string) []string {
		out := slices.Clone(caps)
		slices.Sort(out)
		return out
	}

	t.Run("names", func(t *testing.T) {
		assert.Equal(t, []string{
			"host-volume-create", "host-volume-register", "host-volume-read",
			"host-volume-write", "host-volume-delete",
		}, all)
	})

	const (
		create   = NamespaceCapabilityHostVolumeCreate
		register = NamespaceCapabilityHostVolumeRegister
		read     = NamespaceCapabilityHostVolumeRead
		write    = NamespaceCapabilityHostVolumeWrite
		del      = NamespaceCapabilityHostVolumeDelete
	)

	// The host volume capabilities a policy must end up with, for each way of
	// writing it.
	cases := []struct {
		name string
		hcl  string
		want []string
	}{
		{"read shorthand", `namespace "a" { policy = "read" }`, []string{read}},
		{"write shorthand", `namespace "a" { policy = "write" }`, sorted(all...)},
		{"scale shorthand", `namespace "a" { policy = "scale" }`, nil},
		{"deny shorthand", `namespace "a" { policy = "deny" }`, nil},
		{"write capability", `namespace "a" { capabilities = ["host-volume-write"] }`, sorted(all...)},
		{"register capability", `namespace "a" { capabilities = ["host-volume-register"] }`, sorted(register, create, read)},
		{"create capability", `namespace "a" { capabilities = ["host-volume-create"] }`, sorted(create, read)},
		{"delete capability", `namespace "a" { capabilities = ["host-volume-delete"] }`, sorted(del)},
		{"read capability", `namespace "a" { capabilities = ["host-volume-read"] }`, sorted(read)},
		{"unrelated capability", `namespace "a" { capabilities = ["read-job"] }`, nil},
		{"delete plus create", `namespace "a" { capabilities = ["host-volume-delete", "host-volume-create"] }`, sorted(del, create, read)},
		{"read shorthand plus create", `namespace "a" { policy = "read" capabilities = ["host-volume-create"] }`, sorted(read, create)},
		{"read shorthand plus delete", `namespace "a" { policy = "read" capabilities = ["host-volume-delete"] }`, sorted(read, del)},
		{"scale shorthand plus register", `namespace "a" { policy = "scale" capabilities = ["host-volume-register"] }`, sorted(register, create, read)},
		{"read shorthand plus write", `namespace "a" { policy = "read" capabilities = ["host-volume-write"] }`, sorted(all...)},
		{"write shorthand plus delete", `namespace "a" { policy = "write" capabilities = ["host-volume-delete"] }`, sorted(all...)},
		{"all five listed", `namespace "a" { capabilities = ["host-volume-create", "host-volume-register", "host-volume-read", "host-volume-write", "host-volume-delete"] }`, sorted(all...)},
		{"node policy", `node { policy = "write" }`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse(tc.hcl)
			assert.NoError(t, err)
			var caps []string
			if len(p.Namespaces) > 0 {
				caps = p.Namespaces[0].Capabilities
			}
			assert.Equal(t, tc.want, hvOnly(caps))
			// No capability may appear twice.
			seen := map[string]bool{}
			for _, c := range caps {
				assert.False(t, seen[c], "duplicate %s in %v", c, caps)
				seen[c] = true
			}
		})
	}

	t.Run("deny wins alongside implied capabilities", func(t *testing.T) {
		for _, raw := range []string{
			`namespace "a" { capabilities = ["host-volume-write", "deny"] }`,
			`namespace "a" { policy = "deny" capabilities = ["host-volume-write"] }`,
		} {
			p, err := Parse(raw)
			assert.NoError(t, err)
			assert.Contains(t, p.Namespaces[0].Capabilities, NamespaceCapabilityDeny)
		}
	})

	t.Run("shorthand plus explicit capabilities keeps other capabilities", func(t *testing.T) {
		p, err := Parse(`namespace "a" { policy = "write" capabilities = ["host-volume-read"] }`)
		assert.NoError(t, err)
		assert.Contains(t, p.Namespaces[0].Capabilities, NamespaceCapabilitySubmitJob)
	})

	t.Run("non host volume capabilities are untouched", func(t *testing.T) {
		p, err := Parse(`namespace "a" { policy = "scale" capabilities = ["read-job", "read-job"] }`)
		assert.NoError(t, err)
		n := 0
		for _, c := range p.Namespaces[0].Capabilities {
			if c == NamespaceCapabilityReadJob {
				n++
			}
		}
		assert.Equal(t, 2, n)
	})

	t.Run("unknown capabilities are rejected", func(t *testing.T) {
		for _, capName := range []string{"host-volume-list", "csi-host-volume", "host-volume-mount", "host-volume"} {
			_, err := Parse(fmt.Sprintf(`namespace "team-a" { capabilities = [%q] }`, capName))
			if assert.Error(t, err, capName) {
				assert.Contains(t, err.Error(), "Invalid namespace capability", capName)
			}
		}
	})

	t.Run("namespace capabilities are not host_volume block capabilities", func(t *testing.T) {
		for _, capName := range all {
			_, err := Parse(fmt.Sprintf(`host_volume "data" { capabilities = [%q] }`, capName))
			if assert.Error(t, err, capName) {
				assert.Contains(t, err.Error(), "Invalid host volume capability", capName)
			}
		}
	})

	t.Run("mount capabilities are not namespace capabilities", func(t *testing.T) {
		for _, capName := range []string{HostVolumeCapabilityMountReadOnly, HostVolumeCapabilityMountReadWrite} {
			_, err := Parse(fmt.Sprintf(`namespace "team-a" { capabilities = [%q] }`, capName))
			assert.Error(t, err, capName)
		}
	})
}
