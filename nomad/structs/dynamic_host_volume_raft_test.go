// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// failingImporter lets the type checker proceed without the imports: the
// MessageType constants depend on none of them, and errors from the code that
// does are ignored.
type failingImporter struct{}

func (failingImporter) Import(path string) (*types.Package, error) {
	return nil, fmt.Errorf("import %q is not needed to evaluate message types", path)
}

// declaredMessageTypes type checks the non-test files of this package and
// returns the evaluated value of every MessageType constant, including those
// numbered by iota, so a new constant cannot silently reuse a number.
func declaredMessageTypes(t *testing.T) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)
	pkg, ok := pkgs["structs"]
	require.True(t, ok)
	files := make([]*ast.File, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		files = append(files, file)
	}
	conf := types.Config{Importer: failingImporter{}, Error: func(error) {}, FakeImportC: true}
	checked, _ := conf.Check("structs", fset, files, nil)
	require.NotNil(t, checked)

	found := map[string]int{}
	for _, name := range checked.Scope().Names() {
		constant, ok := checked.Scope().Lookup(name).(*types.Const)
		if !ok {
			continue
		}
		named, ok := constant.Type().(*types.Named)
		if !ok || named.Obj().Name() != "MessageType" {
			continue
		}
		number, exact := constantInt(constant.Val())
		require.True(t, exact, "%s must evaluate to an integer", name)
		found[name] = number
	}
	return found
}

func constantInt(value constant.Value) (int, bool) {
	number, exact := constant.Int64Val(constant.ToInt(value))
	return int(number), exact
}

func TestDynamicHostVolumeMessageTypes(t *testing.T) {
	declared := declaredMessageTypes(t)
	require.Contains(t, declared, "NamespaceDeleteRequestType", "the checker must see the inherited constants")
	require.Contains(t, declared, "NodeRegisterRequestType", "the checker must evaluate iota-numbered constants")

	wanted := map[string]MessageType{
		"DynamicHostVolumeUpsertRequestType":       DynamicHostVolumeUpsertRequestType,
		"DynamicHostVolumeOutcomeRequestType":      DynamicHostVolumeOutcomeRequestType,
		"DynamicHostVolumeUpdateRequestType":       DynamicHostVolumeUpdateRequestType,
		"DynamicHostVolumeMarkDeletingRequestType": DynamicHostVolumeMarkDeletingRequestType,
		"DynamicHostVolumeRemoveRequestType":       DynamicHostVolumeRemoveRequestType,
		"DynamicHostVolumeClaimDeleteRequestType":  DynamicHostVolumeClaimDeleteRequestType,
	}
	owners := map[int]string{}
	for name, number := range declared {
		// The test-setup type is a deliberate alias of the ignore flag.
		if name == "MsgTypeTestSetup" {
			continue
		}
		if other, dup := owners[number]; dup {
			t.Errorf("message type %d is declared by both %s and %s", number, other, name)
		}
		owners[number] = name
	}
	for name, msgType := range wanted {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, msgType, MessageType(declared[name]), "evaluated value must match the constant")
			require.Less(t, int(msgType), int(SystemInitializationType))
			require.Zero(t, msgType&IgnoreUnknownTypeFlag, "a new command must not be ignorable by older servers")
			require.Greater(t, int(msgType), int(NamespaceDeleteRequestType))
		})
	}
}
