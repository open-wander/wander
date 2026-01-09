// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ui
// +build !ui

package agent

import (
	assetfs "github.com/elazarl/go-bindata-assetfs"
)

func init() {
	uiEnabled = false
	stubHTML = `<!DOCTYPE html>
<html>
<p>Wander UI is not available in this binary. To get Wander UI do one of the following:</p>
<ul>
<li><a href="https://openwander.org/wander/downloads.html">Download an official release</a></li>
<li>Run <pre>make release</pre> to create your own release binaries.
<li>Run <pre>make dev-ui</pre> to create a development binary with the UI.
</ul>
</html>
`
}

// assetFS is a stub for building Wander without a UI.
func assetFS() *assetfs.AssetFS {
	return nil
}
