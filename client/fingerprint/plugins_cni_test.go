// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestPluginsCNIFingerprint_Fingerprint_present(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: executableCNIPath(t, "./test_fixtures/cni"),
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	attrVlan := f.(*PluginsCNIFingerprint).attribute("vlan")
	must.Eq(t, "v1.2.3", response.Attributes[attrCustom])
	must.Eq(t, "v1.0.2", response.Attributes[attrBridge])
	must.Eq(t, "v1.2.0", response.Attributes[attrVlan])
}

func TestPluginsCNIFingerprint_Fingerprint_multi(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: executableCNIPath(t, "./test_fixtures/cni", "./test_fixtures/cni2"),
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	attrVlan := f.(*PluginsCNIFingerprint).attribute("vlan")
	attrCustom2 := f.(*PluginsCNIFingerprint).attribute("custom2")
	must.Eq(t, "v1.2.3", response.Attributes[attrCustom])
	must.Eq(t, "v1.0.2", response.Attributes[attrBridge])
	must.Eq(t, "v9.9.9", response.Attributes[attrCustom2])
	must.Eq(t, "v1.2.0", response.Attributes[attrVlan])
}

func executableCNIPath(t *testing.T, fixtureDirs ...string) string {
	t.Helper()

	root := t.TempDir()
	paths := make([]string, 0, len(fixtureDirs))
	for _, fixtureDir := range fixtureDirs {
		destination := filepath.Join(root, filepath.Base(fixtureDir))
		must.NoError(t, os.MkdirAll(destination, 0o755))

		entries, err := os.ReadDir(fixtureDir)
		must.NoError(t, err)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			contents, err := os.ReadFile(filepath.Join(fixtureDir, entry.Name()))
			must.NoError(t, err)
			mode := os.FileMode(0o755)
			if strings.HasSuffix(entry.Name(), ".conf") || strings.HasSuffix(entry.Name(), ".conflist") {
				mode = 0o644
			}
			must.NoError(t, os.WriteFile(filepath.Join(destination, entry.Name()), contents, mode))
		}
		paths = append(paths, destination)
	}
	return strings.Join(paths, string(os.PathListSeparator))
}

func TestPluginsCNIFingerprint_Fingerprint_absent(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "/does/not/exist",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.False(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	must.MapNotContainsKeys(t, response.Attributes, []string{attrCustom, attrBridge})
}

func TestPluginsCNIFingerprint_Fingerprint_empty(t *testing.T) {
	ci.Parallel(t)

	lister := func(string) ([]os.DirEntry, error) {
		// return an empty slice of directory entries
		// i.e. no plugins present
		return nil, nil
	}

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	f.(*PluginsCNIFingerprint).lister = lister
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "./test_fixtures/cni",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
}

func TestPluginsCNIFingerprint_Fingerprint_unset(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: new(config.Config),
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.False(t, response.Detected)
}
