// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package resolvconf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"
)

func TestParse(t *testing.T) {
	config := Parse([]byte(`
# generated configuration
nameserver 1.1.1.1
nameserver invalid
nameserver 2001:4860:4860::8888 # IPv6
search ignored.example
search example.test internal.test
options timeout:1
options attempts:3 rotate
`))

	must.Eq(t, []string{"1.1.1.1", "2001:4860:4860::8888"}, config.Nameservers)
	must.Eq(t, []string{"example.test", "internal.test"}, config.Searches)
	must.Eq(t, []string{"attempts:3", "rotate"}, config.Options)
}

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	err := Write(path, []string{"1.1.1.1"}, []string{"example.test"}, []string{"rotate"})
	must.NoError(t, err)

	content, err := os.ReadFile(path)
	must.NoError(t, err)
	must.Eq(t, "nameserver 1.1.1.1\nsearch example.test\noptions rotate\n", string(content))
}

func TestWrite_rejectsInvalidNameserver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resolv.conf")
	err := Write(path, []string{"not-an-address"}, nil, nil)
	must.ErrorContains(t, err, "bad nameserver address")
}
