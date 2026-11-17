// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/client/hostvolume"
)

const (
	// hostVolumePluginAttribute is the node attribute prefix under which a
	// healthy dynamic host volume plugin is advertised, as
	// plugins.host_volume.version.<id> = <version>. Absence means the
	// plugin is unhealthy or not installed.
	hostVolumePluginAttribute = "plugins.host_volume.version."

	// hostVolumePluginPeriod is how often plugin health is re-checked.
	hostVolumePluginPeriod = time.Minute

	// hostVolumePluginWorkers bounds how many plugins run concurrently in
	// one pass. With hostvolume.MaxCandidates plugins and the default
	// timeout the worst case pass takes MaxCandidates/workers timeouts.
	hostVolumePluginWorkers = 4
)

// PluginsHostVolumeFingerprint advertises the healthy dynamic host volume
// plugins found in the client's host volume plugin directory.
type PluginsHostVolumeFingerprint struct {
	logger hclog.Logger
	lister hostvolume.Lister
	runner hostvolume.Runner

	// lastDiag remembers the last diagnostic reason per plugin so that a
	// permanently broken plugin is logged once, not every period.
	mu       sync.Mutex
	lastDiag map[string]string
}

func NewPluginsHostVolumeFingerprint(logger hclog.Logger) Fingerprint {
	return &PluginsHostVolumeFingerprint{
		logger:   logger.Named("host_volume_plugins"),
		lister:   os.ReadDir,
		lastDiag: map[string]string{},
	}
}

// Periodic makes plugin health changes visible on the next pass.
func (f *PluginsHostVolumeFingerprint) Periodic() (bool, time.Duration) {
	return true, hostVolumePluginPeriod
}

// Fingerprint never returns an error: plugin, directory and listing problems
// are logged as bounded diagnostics, because an error from a periodic
// fingerprinter stops it from running again.
func (f *PluginsHostVolumeFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	pluginDir, volumesDir := "", ""
	if req.Config != nil {
		pluginDir, volumesDir = req.Config.HostVolumePluginDir, req.Config.HostVolumesDir
	}
	builtin := volumesDir != "" && hostvolume.BuiltinSupported()
	resp.Detected = pluginDir != "" || builtin

	var cands []hostvolume.Candidate
	seen := map[string]string{}
	if pluginDir != "" {
		var diags []hostvolume.Diagnostic
		cands, diags = hostvolume.DiscoverWith(pluginDir, f.lister)
		for _, d := range diags {
			seen[d.ID] = d.Reason
			f.logDiag(d)
		}
	}

	results := make([]hostvolume.Result, len(cands))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(hostVolumePluginWorkers, len(cands)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = f.runner.Fingerprint(context.Background(), cands[i])
			}
		}()
	}
	for i := range cands {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	healthy := map[string]string{}
	// The built-in plugin is in-process, so it is healthy whenever the
	// managed volumes directory is configured. An external file named mkdir
	// is excluded in discovery and never reaches here.
	if builtin {
		k := hostVolumePluginAttribute + hostvolume.BuiltinMkdirID
		healthy[k] = hostvolume.BuiltinMkdirVersion
		resp.AddAttribute(k, hostvolume.BuiltinMkdirVersion)
	}
	for _, r := range results {
		if r.Healthy {
			healthy[hostVolumePluginAttribute+r.ID] = r.Version
			resp.AddAttribute(hostVolumePluginAttribute+r.ID, r.Version)
			continue
		}
		if r.Diag != nil {
			seen[r.ID] = r.Diag.Reason
			f.logDiag(*r.Diag)
		}
	}

	// Removal is driven by a scan of the node's current attributes, so it is
	// correct after a client restart and needs no state of its own.
	if req.Node != nil {
		for k := range req.Node.Attributes {
			if !strings.HasPrefix(k, hostVolumePluginAttribute) {
				continue
			}
			if _, ok := healthy[k]; !ok {
				resp.RemoveAttribute(k)
			}
		}
	}

	f.mu.Lock()
	f.lastDiag = seen
	f.mu.Unlock()
	return nil
}

func (f *PluginsHostVolumeFingerprint) logDiag(d hostvolume.Diagnostic) {
	f.mu.Lock()
	prev := f.lastDiag[d.ID]
	f.mu.Unlock()
	// An unsupported platform is expected on every such client, not a fault.
	if prev == d.Reason || d.Reason == hostvolume.ReasonUnsupportedOS {
		f.logger.Debug("host volume plugin excluded", "plugin", d.ID, "reason", d.Reason)
		return
	}
	f.logger.Warn("host volume plugin excluded", "plugin", d.ID, "reason", d.Reason, "detail", d.Detail)
}
