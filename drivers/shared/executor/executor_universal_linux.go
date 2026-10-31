// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/open-wander/wander/client/lib/cgutil"
	"github.com/open-wander/wander/client/lib/resources"
	"github.com/open-wander/wander/client/taskenv"
	"github.com/open-wander/wander/helper/users"
	"github.com/open-wander/wander/plugins/drivers"
	"github.com/opencontainers/cgroups"
	_ "github.com/opencontainers/cgroups/devices"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

// setCmdUser takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func setCmdUser(cmd *exec.Cmd, userid string) error {
	credential, err := lookupUserCredential(userid)
	if err != nil {
		return err
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = credential

	return nil
}

func lookupUserCredential(userid string) (*syscall.Credential, error) {
	u, err := users.Lookup(userid)
	if err != nil {
		return nil, fmt.Errorf("failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, 0, len(gidStrings))
	for _, gidString := range gidStrings {
		gid, err := parseUserID(gidString, "supplementary group")
		if err != nil {
			return nil, err
		}
		gids = append(gids, gid)
	}

	uid, err := parseUserID(u.Uid, "user ID")
	if err != nil {
		return nil, err
	}
	gid, err := parseUserID(u.Gid, "group ID")
	if err != nil {
		return nil, err
	}

	return &syscall.Credential{
		Uid:    uid,
		Gid:    gid,
		Groups: gids,
	}, nil
}

func parseUserID(value, label string) (uint32, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("unable to convert %s %q to uint32: %w", label, value, err)
	}
	return uint32(id), nil
}

// configureResourceContainer configured the cgroups to be used to track pids
// created by the executor
func (e *UniversalExecutor) configureResourceContainer(pid int) error {
	cfg := &configs.Config{
		Cgroups: &cgroups.Cgroup{
			Resources: &cgroups.Resources{},
		},
	}

	// note: this was always here, but not used until cgroups v2 support
	for _, device := range specconv.AllowedDevices {
		cfg.Cgroups.Resources.Devices = append(cfg.Cgroups.Resources.Devices, &device.Rule)
	}

	lookup := func(env []string, name string) (result string) {
		for _, s := range env {
			if strings.HasPrefix(s, name+"=") {
				result = strings.TrimLeft(s, name+"=")
				return
			}
		}
		return
	}

	if cgutil.UseV2 {
		// in v2 we have the definitive cgroup; create and enter it

		// use the task environment variables for determining the cgroup path -
		// not ideal but plumbing the values directly requires grpc protobuf changes
		parent := lookup(e.commandCfg.Env, taskenv.CgroupParent)
		allocID := lookup(e.commandCfg.Env, taskenv.AllocID)
		task := lookup(e.commandCfg.Env, taskenv.TaskName)
		if parent == "" || allocID == "" || task == "" {
			return fmt.Errorf(
				"environment variables %s must be set",
				strings.Join([]string{taskenv.CgroupParent, taskenv.AllocID, taskenv.TaskName}, ","),
			)
		}
		scope := cgutil.CgroupScope(allocID, task)
		path := filepath.Join("/", cgutil.GetCgroupParent(parent), scope)
		cfg.Cgroups.Path = path
		e.containment = resources.Contain(e.logger, cfg.Cgroups)
		return e.containment.Apply(pid)

	} else {
		// in v1 create a freezer cgroup for use by containment

		if err := cgutil.ConfigureBasicCgroups(cfg); err != nil {
			// Log this error to help diagnose cases where nomad is run with too few
			// permissions, but don't return an error. There is no separate check for
			// cgroup creation permissions, so this may be the happy path.
			e.logger.Warn("failed to create cgroup",
				"docs", "https://openwander.org/wander/docs/drivers/raw_exec.html#no_cgroups",
				"error", err)
			return nil
		}
		path := cfg.Cgroups.Path
		e.logger.Trace("cgroup created, now need to apply", "path", path)
		e.containment = resources.Contain(e.logger, cfg.Cgroups)
		return e.containment.Apply(pid)
	}
}

func (e *UniversalExecutor) getAllPids() (resources.PIDs, error) {
	if e.containment == nil {
		return getAllPidsByScanning()
	}
	return e.containment.GetPIDs(), nil
}

// withNetworkIsolation calls the passed function the network namespace `spec`
func withNetworkIsolation(f func() error, spec *drivers.NetworkIsolationSpec) error {
	if spec != nil && spec.Path != "" {
		// Get a handle to the target network namespace
		netNS, err := ns.GetNS(spec.Path)
		if err != nil {
			return err
		}

		// Start the container in the network namespace
		return netNS.Do(func(ns.NetNS) error {
			return f()
		})
	}
	return f()
}
