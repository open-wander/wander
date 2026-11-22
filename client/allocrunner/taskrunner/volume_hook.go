// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"slices"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/open-wander/wander/client/allocrunner/interfaces"
	"github.com/open-wander/wander/client/taskenv"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/plugins/drivers"
)

// errHostVolumeWaitCancelled reports that the wait for the volume restore
// ended because the task is being killed or the agent is shutting down.
var errHostVolumeWaitCancelled = errors.New("wait for host volumes cancelled")

type volumeHook struct {
	alloc   *structs.Allocation
	runner  *TaskRunner
	logger  log.Logger
	taskEnv *taskenv.TaskEnv
}

func newVolumeHook(runner *TaskRunner, logger log.Logger) *volumeHook {
	h := &volumeHook{
		alloc:  runner.Alloc(),
		runner: runner,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*volumeHook) Name() string {
	return "volumes"
}

// missingHostVolumes returns the sorted sources of the requested host volumes
// that the client does not offer. Only sources are returned, never paths.
func missingHostVolumes(requestedByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig, allocName string) []string {
	var missing []string

	for _, req := range requestedByAlias {
		// This is a defensive check, but this function should only ever receive
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		source := req.Source
		if req.PerAlloc {
			source = source + structs.AllocSuffix(allocName)
		}

		if _, ok := clientVolumesByName[source]; !ok {
			missing = append(missing, source)
		}
	}

	slices.Sort(missing)
	return missing
}

func validateHostVolumes(requestedByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig, allocName string) error {
	var result error

	for _, source := range missingHostVolumes(requestedByAlias, clientVolumesByName, allocName) {
		result = multierror.Append(result, fmt.Errorf("missing %s", source))
	}

	return result
}

// hostVolumeMountConfigurations takes the users requested volume mounts,
// volumes, and the client host volume configuration and converts them into a
// format that can be used by drivers.
func (h *volumeHook) hostVolumeMountConfigurations(taskMounts []*structs.VolumeMount, taskVolumesByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig, allocName string) ([]*drivers.MountConfig, error) {
	var mounts []*drivers.MountConfig
	for _, m := range taskMounts {
		req, ok := taskVolumesByAlias[m.Volume]
		if !ok {
			// This function receives only the task volumes that are of type Host,
			// if we can't find a group volume then we assume the mount is for another
			// type.
			continue
		}

		// This is a defensive check, but this function should only ever receive
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		source := req.Source
		if req.PerAlloc {
			source = source + structs.AllocSuffix(allocName)
		}
		hostVolume, ok := clientVolumesByName[source]
		if !ok {
			// Should never happen, but unless the client volumes were mutated during
			// the execution of this hook.
			return nil, fmt.Errorf("no host volume named: %s", source)
		}

		mcfg := &drivers.MountConfig{
			HostPath:        hostVolume.Path,
			TaskPath:        m.Destination,
			Readonly:        hostVolume.ReadOnly || req.ReadOnly || m.ReadOnly,
			PropagationMode: m.PropagationMode,
		}
		mounts = append(mounts, mcfg)
	}

	return mounts, nil
}

// partitionVolumesByType takes a map of volume-alias to volume-request and
// returns them in the form of volume-type:(volume-alias:volume-request)
func partitionVolumesByType(xs map[string]*structs.VolumeRequest) map[string]map[string]*structs.VolumeRequest {
	result := make(map[string]map[string]*structs.VolumeRequest)
	for name, req := range xs {
		txs, ok := result[req.Type]
		if !ok {
			txs = make(map[string]*structs.VolumeRequest)
			result[req.Type] = txs
		}
		txs[name] = req
	}

	return result
}

// liveHostVolumes returns the host volumes of the client that the allocation
// may mount. Without an injected getter it is the set captured when the task
// runner was built, as for static volumes only.
//
// Wander policy: dynamic host volumes are published after the task runner is
// built, and after an agent restart a restored allocation can reach prestart
// before the volume manager has restored them. When a requested source is
// missing the hook therefore waits for the first restore to end, bounded by
// the restore timeout and by ctx, and looks again. It does not return a
// recoverable error instead: prestart also runs for a task that is still
// running after the restart, and a failed restart decision would kill it
// because of start ordering. A static name that is not configured is delayed
// by the same wait and then fails with the same error. A task whose volumes
// are all present never waits.
//
// A cancelled ctx (a job stop, or the agent stopping again while the restore
// runs) is not a failure of the volumes: it returns
// errHostVolumeWaitCancelled, which Prestart turns into a nil return without
// mounts and without marking the hook done. The task runner then sees its own
// cancelled context, so a task is killed or left for the next agent start
// instead of being failed with a hook error, and the hook runs again on the
// next prestart. Cancellation wins over a restore that ends at the same time.
func (h *volumeHook) liveHostVolumes(ctx context.Context, req *interfaces.TaskPrestartRequest, volumes map[string]*structs.VolumeRequest) (map[string]*structs.ClientHostVolumeConfig, error) {
	if h.runner.hostVolumes == nil {
		return h.runner.clientConfig.Node.HostVolumes, nil
	}

	ns := req.Alloc.Namespace
	hostVolumes := h.runner.hostVolumes(ns)
	if h.runner.hostVolumesRestored == nil || len(missingHostVolumes(volumes, hostVolumes, req.Alloc.Name)) == 0 {
		return hostVolumes, nil
	}

	select {
	case <-h.runner.hostVolumesRestored():
	case <-ctx.Done():
	}
	if ctx.Err() != nil {
		return nil, errHostVolumeWaitCancelled
	}
	return h.runner.hostVolumes(ns), nil
}

func (h *volumeHook) prepareHostVolumes(ctx context.Context, req *interfaces.TaskPrestartRequest, volumes map[string]*structs.VolumeRequest) ([]*drivers.MountConfig, error) {
	hostVolumes, err := h.liveHostVolumes(ctx, req, volumes)
	if err != nil {
		return nil, err
	}

	// Always validate volumes to ensure that we do not allow volumes to be used
	// if a host is restarted and loses the host volume configuration.
	if err := validateHostVolumes(volumes, hostVolumes, req.Alloc.Name); err != nil {
		// Only the missing sources are logged: the set of the client holds
		// paths, including those of volumes of other namespaces.
		missing := missingHostVolumes(volumes, hostVolumes, req.Alloc.Name)
		h.logger.Error("Requested Host Volume does not exist", "missing", missing, "missing_count", len(missing))
		return nil, fmt.Errorf("host volume validation error: %v", err)
	}

	hostVolumeMounts, err := h.hostVolumeMountConfigurations(req.Task.VolumeMounts, volumes, hostVolumes, req.Alloc.Name)
	if err != nil {
		h.logger.Error("Failed to generate host volume mounts", "error", err)
		return nil, err
	}

	if len(hostVolumeMounts) > 0 {
		caps, err := h.runner.DriverCapabilities()
		if err != nil {
			return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
		}
		if caps.MountConfigs == drivers.MountConfigSupportNone {
			return nil, fmt.Errorf(
				"task driver %q for %q does not support host volumes",
				h.runner.task.Driver, h.runner.task.Name)
		}
	}

	return hostVolumeMounts, nil
}

// partitionMountsByVolume takes a list of volume mounts and returns them in the
// form of volume-alias:[]volume-mount because one volume may be mounted multiple
// times.
func partitionMountsByVolume(xs []*structs.VolumeMount) map[string][]*structs.VolumeMount {
	result := make(map[string][]*structs.VolumeMount)
	for _, mount := range xs {
		result[mount.Volume] = append(result[mount.Volume], mount)
	}

	return result
}

func (h *volumeHook) prepareCSIVolumes(req *interfaces.TaskPrestartRequest, volumes map[string]*structs.VolumeRequest) ([]*drivers.MountConfig, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	var mounts []*drivers.MountConfig

	mountRequests := partitionMountsByVolume(req.Task.VolumeMounts)
	csiMountPoints := h.runner.allocHookResources.GetCSIMounts()
	for alias, request := range volumes {
		mountsForAlias, ok := mountRequests[alias]
		if !ok {
			// This task doesn't use the volume
			continue
		}

		csiMountPoint, ok := csiMountPoints[alias]
		if !ok {
			return nil, fmt.Errorf("No CSI Mount Point found for volume: %s", alias)
		}

		for _, m := range mountsForAlias {
			mcfg := &drivers.MountConfig{
				HostPath:        csiMountPoint.Source,
				TaskPath:        m.Destination,
				Readonly:        request.ReadOnly || m.ReadOnly,
				PropagationMode: m.PropagationMode,
			}
			mounts = append(mounts, mcfg)
		}
	}

	if len(mounts) > 0 {
		caps, err := h.runner.DriverCapabilities()
		if err != nil {
			return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
		}
		if caps.MountConfigs == drivers.MountConfigSupportNone {
			return nil, fmt.Errorf(
				"task driver %q for %q does not support CSI",
				h.runner.task.Driver, h.runner.task.Name)
		}
	}

	return mounts, nil
}

func (h *volumeHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.taskEnv = req.TaskEnv
	interpolateVolumeMounts(req.Task.VolumeMounts, h.taskEnv)

	volumes := partitionVolumesByType(h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).Volumes)

	hostVolumeMounts, err := h.prepareHostVolumes(ctx, req, volumes[structs.VolumeTypeHost])
	if err != nil {
		if errors.Is(err, errHostVolumeWaitCancelled) {
			return nil
		}
		return err
	}

	csiVolumeMounts, err := h.prepareCSIVolumes(req, volumes[structs.VolumeTypeCSI])
	if err != nil {
		return err
	}

	// Because this hook is also ran on restores, we only add mounts that do not
	// already exist. Although this loop is somewhat expensive, there are only
	// a small number of mounts that exist within most individual tasks. We may
	// want to revisit this using a `hookdata` param to be "mount only once"
	mounts := h.runner.hookResources.getMounts()
	for _, m := range hostVolumeMounts {
		mounts = ensureMountpointInserted(mounts, m)
	}
	for _, m := range csiVolumeMounts {
		mounts = ensureMountpointInserted(mounts, m)
	}
	h.runner.hookResources.setMounts(mounts)

	return nil
}

func interpolateVolumeMounts(mounts []*structs.VolumeMount, taskEnv *taskenv.TaskEnv) {
	for _, mount := range mounts {
		mount.Volume = taskEnv.ReplaceEnv(mount.Volume)
		mount.Destination = taskEnv.ReplaceEnv(mount.Destination)
		mount.PropagationMode = taskEnv.ReplaceEnv(mount.PropagationMode)
	}
}
