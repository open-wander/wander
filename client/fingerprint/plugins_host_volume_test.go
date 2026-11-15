// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package fingerprint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/hostvolume"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func hvPlugin(t *testing.T, dir, name, body string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(dir, name)
	must.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), mode))
	must.NoError(t, os.Chmod(p, mode))
}

func newHVFingerprinter(t *testing.T) *PluginsHostVolumeFingerprint {
	f := NewPluginsHostVolumeFingerprint(testlog.HCLogger(t)).(*PluginsHostVolumeFingerprint)
	f.runner = hostvolume.Runner{
		Timeout: 3 * time.Second,
		Limits:  hostvolume.Limits{Stdout: 128, Stderr: 128},
	}
	return f
}

func runHV(t *testing.T, f *PluginsHostVolumeFingerprint, dir string, node *structs.Node) *FingerprintResponse {
	t.Helper()
	req := &FingerprintRequest{Config: &config.Config{HostVolumePluginDir: dir}, Node: node}
	resp := new(FingerprintResponse)
	must.NoError(t, f.Fingerprint(req, resp))
	return resp
}

func TestPluginsHostVolumeFingerprint_Periodic(t *testing.T) {
	ci.Parallel(t)
	periodic, d := newHVFingerprinter(t).Periodic()
	must.True(t, periodic)
	must.Positive(t, d)
	_, err := NewFingerprint("plugins_host_volume", testlog.HCLogger(t))
	must.NoError(t, err)
}

func TestPluginsHostVolumeFingerprint_Unset(t *testing.T) {
	ci.Parallel(t)
	f := newHVFingerprinter(t)
	resp := runHV(t, f, "", nil)
	must.False(t, resp.Detected)
	must.MapLen(t, 0, resp.Attributes)
}

func TestPluginsHostVolumeFingerprint_EmptyAndMissing(t *testing.T) {
	ci.Parallel(t)
	for name, dir := range map[string]string{
		"empty":   t.TempDir(),
		"missing": filepath.Join(t.TempDir(), "nope"),
	} {
		resp := runHV(t, newHVFingerprinter(t), dir, nil)
		must.True(t, resp.Detected, must.Sprint(name))
		must.MapLen(t, 0, resp.Attributes, must.Sprint(name))
	}
}

func TestPluginsHostVolumeFingerprint_Advertise(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	hvPlugin(t, dir, "alpha", `echo '{"version":"1.2.3"}'`, 0o755)
	hvPlugin(t, dir, "beta", `echo '{"version":"v2"}'`, 0o755)
	// excluded in five different ways
	hvPlugin(t, dir, "noexec", `echo '{"version":"1"}'`, 0o644)
	hvPlugin(t, dir, "malformed", `echo '{"version":"a b"}'`, 0o755)
	hvPlugin(t, dir, "failing", "exit 1", 0o755)
	hvPlugin(t, dir, "chatty", "exec yes 0123456789", 0o755)
	hvPlugin(t, dir, "slow", "exec sleep 30", 0o755)
	must.NoError(t, os.Mkdir(filepath.Join(dir, "adir"), 0o755))

	f := newHVFingerprinter(t)
	f.runner.Timeout = 500 * time.Millisecond
	// give the healthy plugins room even on a loaded machine
	start := time.Now()
	resp := runHV(t, f, dir, nil)
	must.Less(t, 10*time.Second, time.Since(start))
	must.True(t, resp.Detected)
	must.Eq(t, map[string]string{
		"plugins.host_volume.version.alpha": "1.2.3",
		"plugins.host_volume.version.beta":  "v2",
	}, resp.Attributes)
}

func TestPluginsHostVolumeFingerprint_NeverReturnsError(t *testing.T) {
	ci.Parallel(t)
	f := newHVFingerprinter(t)
	f.lister = func(string) ([]os.DirEntry, error) { return nil, errors.New("boom \x1b[31m") }
	resp := new(FingerprintResponse)
	err := f.Fingerprint(&FingerprintRequest{Config: &config.Config{HostVolumePluginDir: "/x"}}, resp)
	must.NoError(t, err)
	must.MapLen(t, 0, resp.Attributes)
}

func TestPluginsHostVolumeFingerprint_HealthChange(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	hvPlugin(t, dir, "alpha", `echo '{"version":"1.0"}'`, 0o755)
	hvPlugin(t, dir, "beta", `echo '{"version":"2.0"}'`, 0o755)
	f := newHVFingerprinter(t)

	node := &structs.Node{Attributes: map[string]string{"kernel.name": "linux"}}
	resp := runHV(t, f, dir, node)
	must.MapLen(t, 2, resp.Attributes)
	for k, v := range resp.Attributes {
		node.Attributes[k] = v
	}

	// alpha is removed, beta is invalidated, gamma was never there
	must.NoError(t, os.Remove(filepath.Join(dir, "alpha")))
	hvPlugin(t, dir, "beta", "exit 2", 0o755)
	// a stale attribute for a plugin that no longer exists, as after a restart
	node.Attributes["plugins.host_volume.version.ghost"] = "9"

	resp = runHV(t, f, dir, node)
	must.Eq(t, map[string]string{
		"plugins.host_volume.version.alpha": "",
		"plugins.host_volume.version.beta":  "",
		"plugins.host_volume.version.ghost": "",
	}, resp.Attributes)
	_, touched := resp.Attributes["kernel.name"]
	must.False(t, touched)

	// beta recovers
	hvPlugin(t, dir, "beta", `echo '{"version":"2.1"}'`, 0o755)
	resp = runHV(t, f, dir, node)
	must.Eq(t, "2.1", resp.Attributes["plugins.host_volume.version.beta"])
}

func TestPluginsHostVolumeFingerprint_ShuffledListing(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		hvPlugin(t, dir, n, `echo '{"version":"1"}'`, 0o755)
	}
	real, err := os.ReadDir(dir)
	must.NoError(t, err)

	f1 := newHVFingerprinter(t)
	f2 := newHVFingerprinter(t)
	f2.lister = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{real[2], real[0], real[1]}, nil
	}
	r1 := runHV(t, f1, dir, nil)
	r2 := runHV(t, f2, dir, nil)
	must.Eq(t, r1.Attributes, r2.Attributes)
	must.MapLen(t, 3, r1.Attributes)
}
