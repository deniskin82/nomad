// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	ci.Parallel(t)

	type tcase struct {
		Raw       string
		ExpectErr string
		Expect    *Policy
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
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRead,
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
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRead,
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
			  variables {
                path "/nomad/job" {
			      capabilities = ["read", "write"]
                }
			  }
			}
			`,
			"Invalid variable path \"/nomad/job\" in namespace dev: cannot start with a leading '/'",
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
			namespace "default" {
				capabilities = ["host-volume-register"]
			}

			namespace "other" {
				capabilities = ["host-volume-create"]
			}

			namespace "foo" {
				capabilities = ["host-volume-write"]
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: "",
						Capabilities: []string{
							NamespaceCapabilityHostVolumeRegister,
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRead,
						},
					},
					{
						Name:   "other",
						Policy: "",
						Capabilities: []string{
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeRead,
						},
					},
					{
						Name:   "foo",
						Policy: "",
						Capabilities: []string{
							NamespaceCapabilityHostVolumeWrite,
							NamespaceCapabilityHostVolumeRegister,
							NamespaceCapabilityHostVolumeCreate,
							NamespaceCapabilityHostVolumeDelete,
							NamespaceCapabilityHostVolumeRead,
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
		t.Run(fmt.Sprintf("%02d", idx), func(t *testing.T) {
			p, err := Parse(tc.Raw)
			if tc.ExpectErr == "" {
				must.NoError(t, err)
			} else {
				must.ErrorContains(t, err, tc.ExpectErr)
			}

			if tc.Expect != nil {
				tc.Expect.Raw = tc.Raw
				must.Eq(t, tc.Expect, p)
			}
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

func TestPluginPolicy_isValid(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name              string
		inputPluginPolicy *PluginPolicy
		expectedOutput    bool
	}{
		{
			name:              "policy deny",
			inputPluginPolicy: &PluginPolicy{Policy: "deny"},
			expectedOutput:    true,
		},
		{
			name:              "policy read",
			inputPluginPolicy: &PluginPolicy{Policy: "read"},
			expectedOutput:    true,
		},
		{
			name:              "policy list",
			inputPluginPolicy: &PluginPolicy{Policy: "list"},
			expectedOutput:    true,
		},
		{
			name:              "policy write",
			inputPluginPolicy: &PluginPolicy{Policy: "write"},
			expectedOutput:    true,
		},
		{
			name:              "policy invalid",
			inputPluginPolicy: &PluginPolicy{Policy: "invalid"},
			expectedOutput:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputPluginPolicy.isValid()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}
