// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resolvconf

import (
	"io"
	"os"
	"path/filepath"

	"github.com/open-wander/wander/plugins/drivers"
)

func GenerateDNSMount(taskDir string, conf *drivers.DNSConfig) (*drivers.MountConfig, error) {
	var nSearches, nServers, nOptions int
	path := filepath.Join(taskDir, "resolv.conf")
	mount := &drivers.MountConfig{
		TaskPath:        "/etc/resolv.conf",
		HostPath:        path,
		Readonly:        true,
		PropagationMode: "private",
	}
	if conf != nil {
		nServers = len(conf.Servers)
		nSearches = len(conf.Searches)
		nOptions = len(conf.Options)
	}

	// Use system dns if no configuration is given
	if nServers == 0 && nSearches == 0 && nOptions == 0 {
		if err := copySystemDNS(path); err != nil {
			return nil, err
		}

		return mount, nil
	}

	currRC, err := Read(SystemPath())
	if err != nil {
		return nil, err
	}

	var (
		dnsList        = currRC.Nameservers
		dnsSearchList  = currRC.Searches
		dnsOptionsList = currRC.Options
	)
	if nServers > 0 {
		dnsList = conf.Servers
	}
	if nSearches > 0 {
		dnsSearchList = conf.Searches
	}
	if nOptions > 0 {
		dnsOptionsList = conf.Options
	}

	if err = Write(path, dnsList, dnsSearchList, dnsOptionsList); err != nil {
		return nil, err
	}

	return mount, nil
}

func copySystemDNS(filePath string) error {
	in, err := os.Open(SystemPath())
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, content, 0644)
}
