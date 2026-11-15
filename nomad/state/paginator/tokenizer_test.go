// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package paginator

import (
	"fmt"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/stretchr/testify/require"
)

func TestStructsTokenizer(t *testing.T) {
	ci.Parallel(t)

	j := mock.Job()

	cases := []struct {
		name     string
		opts     StructsTokenizerOptions
		expected string
	}{
		{
			name: "ID",
			opts: StructsTokenizerOptions{
				WithID: true,
			},
			expected: fmt.Sprintf("%v", j.ID),
		},
		{
			name: "Namespace.ID",
			opts: StructsTokenizerOptions{
				WithNamespace: true,
				WithID:        true,
			},
			expected: fmt.Sprintf("%v.%v", j.Namespace, j.ID),
		},
		{
			name: "CreateIndex.Namespace.ID",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithNamespace:   true,
				WithID:          true,
			},
			expected: fmt.Sprintf("%v.%v.%v", j.CreateIndex, j.Namespace, j.ID),
		},
		{
			name: "CreateIndex.ID",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithID:          true,
			},
			expected: fmt.Sprintf("%v.%v", j.CreateIndex, j.ID),
		},
		{
			name: "CreateIndex.Namespace",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithNamespace:   true,
			},
			expected: fmt.Sprintf("%v.%v", j.CreateIndex, j.Namespace),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := StructsTokenizer{opts: tc.opts}
			require.Equal(t, tc.expected, tokenizer.GetToken(j))
		})
	}
}

// TestStructsTokenizer_Compare proves tokens compare field by field with the
// index compared as a number, and a target in another format as a string.
func TestStructsTokenizer_Compare(t *testing.T) {
	ci.Parallel(t)

	nsID := StructsTokenizerOptions{WithNamespace: true, WithID: true}
	indexID := StructsTokenizerOptions{WithCreateIndex: true, WithID: true}

	cases := []struct {
		name     string
		opts     StructsTokenizerOptions
		obj      *mockObject
		target   string
		expected int
	}{
		{name: "namespace before its dash suffix", opts: nsID, obj: &mockObject{namespace: "team", id: "j3"}, target: "team-a.j1", expected: -1},
		{name: "dash suffix after its namespace", opts: nsID, obj: &mockObject{namespace: "team-a", id: "j1"}, target: "team.j3", expected: 1},
		{name: "same namespace and ID", opts: nsID, obj: &mockObject{namespace: "team", id: "j1"}, target: "team.j1", expected: 0},
		{name: "ID containing a dot", opts: nsID, obj: &mockObject{namespace: "team", id: "a.b"}, target: "team.a.b", expected: 0},
		{name: "index as a number", opts: indexID, obj: &mockObject{createIndex: 9, id: "a"}, target: "10.a", expected: -1},
		{name: "same index then ID", opts: indexID, obj: &mockObject{createIndex: 10, id: "b"}, target: "10.a", expected: 1},
		{name: "non-numeric index as a string", opts: indexID, obj: &mockObject{createIndex: 9, id: "a"}, target: "x.a", expected: -1},
		{name: "missing ID as a string", opts: indexID, obj: &mockObject{createIndex: 9, id: "a"}, target: "10", expected: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := StructsTokenizer{opts: tc.opts}
			require.Equal(t, tc.expected, tokenizer.Compare(tc.obj, tc.target))
		})
	}
}
