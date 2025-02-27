// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"

	log "github.com/hashicorp/go-hclog"

	"github.com/open-wander/wander/client/allocdir"
	arinterfaces "github.com/open-wander/wander/client/allocrunner/interfaces"
	"github.com/open-wander/wander/client/consul"
	"github.com/open-wander/wander/client/devicemanager"
	"github.com/open-wander/wander/client/dynamicplugins"
	"github.com/open-wander/wander/client/interfaces"
	"github.com/open-wander/wander/client/lib/cgutil"
	"github.com/open-wander/wander/client/pluginmanager/csimanager"
	"github.com/open-wander/wander/client/pluginmanager/drivermanager"
	"github.com/open-wander/wander/client/serviceregistration"
	"github.com/open-wander/wander/client/serviceregistration/checks/checkstore"
	"github.com/open-wander/wander/client/serviceregistration/wrapper"
	cstate "github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/client/vaultclient"
	"github.com/open-wander/wander/nomad/structs"
)

// AllocRunnerFactory returns an AllocRunner interface built from the
// configuration. Note: the type for config is any because we can't count on
// test callers being able to make a real allocrunner.Config without an circular
// import
type AllocRunnerFactory func(*AllocRunnerConfig) (arinterfaces.AllocRunner, error)

// RPCer is the interface needed by hooks to make RPC calls.
type RPCer interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// AllocRunnerConfig holds the configuration for creating an allocation runner.
type AllocRunnerConfig struct {
	// Logger is the logger for the allocation runner.
	Logger log.Logger

	// ClientConfig is the clients configuration.
	ClientConfig *Config

	// Alloc captures the allocation that should be run.
	Alloc *structs.Allocation

	// StateDB is used to store and restore state.
	StateDB cstate.StateDB

	// Consul is the Consul client used to register task services and checks
	Consul serviceregistration.Handler

	// ConsulProxies is the Consul client used to lookup supported envoy versions
	// of the Consul agent.
	ConsulProxies consul.SupportedProxiesAPI

	// ConsulSI is the Consul client used to manage service identity tokens.
	ConsulSI consul.ServiceIdentityAPI

	// Vault is the Vault client to use to retrieve Vault tokens
	Vault vaultclient.VaultClient

	// StateUpdater is used to emit updated task state
	StateUpdater interfaces.AllocStateHandler

	// DeviceStatsReporter is used to lookup resource usage for alloc devices
	DeviceStatsReporter interfaces.DeviceStatsReporter

	// PrevAllocWatcher handles waiting on previous or preempted allocations
	PrevAllocWatcher PrevAllocWatcher

	// PrevAllocMigrator allows the migration of a previous allocations alloc dir
	PrevAllocMigrator PrevAllocMigrator

	// DynamicRegistry contains all locally registered dynamic plugins (e.g csi
	// plugins).
	DynamicRegistry dynamicplugins.Registry

	// CSIManager is used to wait for CSI Volumes to be attached, and by the task
	// runner to manage their mounting
	CSIManager csimanager.Manager

	// DeviceManager is used to mount devices as well as lookup device
	// statistics
	DeviceManager devicemanager.Manager

	// DriverManager handles dispensing of driver plugins
	DriverManager drivermanager.Manager

	// CpusetManager configures the cpuset cgroup if supported by the platform
	CpusetManager cgutil.CpusetManager

	// ServersContactedCh is closed when the first GetClientAllocs call to
	// servers succeeds and allocs are synced.
	ServersContactedCh chan struct{}

	// RPCClient is the RPC Client that should be used by the allocrunner and its
	// hooks to communicate with Nomad Servers.
	RPCClient RPCer

	// ServiceRegWrapper is the handler wrapper that is used by service hooks
	// to perform service and check registration and deregistration.
	ServiceRegWrapper *wrapper.HandlerWrapper

	// CheckStore contains check result information.
	CheckStore checkstore.Shim

	// Getter is an interface for retrieving artifacts.
	Getter interfaces.ArtifactGetter
}

// PrevAllocWatcher allows AllocRunners to wait for a previous allocation to
// terminate whether or not the previous allocation is local or remote.
// See `PrevAllocMigrator` for migrating workloads.
type PrevAllocWatcher interface {
	// Wait for previous alloc to terminate
	Wait(context.Context) error

	// IsWaiting returns true if a concurrent caller is blocked in Wait
	IsWaiting() bool
}

// PrevAllocMigrator allows AllocRunners to migrate a previous allocation
// whether or not the previous allocation is local or remote.
type PrevAllocMigrator interface {
	PrevAllocWatcher

	// IsMigrating returns true if a concurrent caller is in Migrate
	IsMigrating() bool

	// Migrate data from previous alloc
	Migrate(ctx context.Context, dest *allocdir.AllocDir) error
}
