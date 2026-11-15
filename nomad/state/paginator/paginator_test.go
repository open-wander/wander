// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package paginator

import (
	"errors"
	"slices"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestPaginator(t *testing.T) {
	ci.Parallel(t)
	ids := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	cases := []struct {
		name              string
		perPage           int32
		nextToken         string
		expected          []string
		expectedNextToken string
		expectedError     string
	}{
		{
			name:              "size-3 page-1",
			perPage:           3,
			expected:          []string{"0", "1", "2"},
			expectedNextToken: "3",
		},
		{
			name:              "size-5 page-2 stop before end",
			perPage:           5,
			nextToken:         "3",
			expected:          []string{"3", "4", "5", "6", "7"},
			expectedNextToken: "8",
		},
		{
			name:              "page-2 reading off the end",
			perPage:           10,
			nextToken:         "5",
			expected:          []string{"5", "6", "7", "8", "9"},
			expectedNextToken: "",
		},
		{
			name:              "starting off the end",
			perPage:           5,
			nextToken:         "a",
			expected:          []string{},
			expectedNextToken: "",
		},
		{
			name:          "error during append",
			expectedError: "failed to append",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			iter := newTestIterator(ids)
			tokenizer := testTokenizer{}
			opts := structs.QueryOptions{
				PerPage:   tc.perPage,
				NextToken: tc.nextToken,
			}

			results := []string{}
			paginator, err := NewPaginator(iter, tokenizer, nil, opts,
				func(raw interface{}) error {
					if tc.expectedError != "" {
						return errors.New(tc.expectedError)
					}

					result := raw.(*mockObject)
					results = append(results, result.id)
					return nil
				},
			)
			require.NoError(t, err)

			nextToken, err := paginator.Page()
			if tc.expectedError == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expected, results)
				require.Equal(t, tc.expectedNextToken, nextToken)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			}
		})
	}

}

// TestPaginator_NamespaceIDOrdering proves pages follow the (Namespace, ID)
// iteration order, where "team" sorts before "team-a" although the token
// "team-a.j1" sorts before "team.j1" as a string.
func TestPaginator_NamespaceIDOrdering(t *testing.T) {
	ci.Parallel(t)

	// (Namespace, ID) order, as memdb iterates the jobs and variables tables
	ascending := []*mockObject{
		{namespace: "team", id: "j1"},
		{namespace: "team", id: "j2"},
		{namespace: "team", id: "j3"},
		{namespace: "team-a", id: "j1"},
		{namespace: "team-a", id: "j2"},
		{namespace: "team-a", id: "j3"},
	}
	tokenOpts := StructsTokenizerOptions{WithNamespace: true, WithID: true}

	t.Run("ascending", func(t *testing.T) {
		requirePagesInOrder(t, ascending, tokenOpts, false)
	})
	t.Run("descending", func(t *testing.T) {
		requirePagesInOrder(t, reversed(ascending), tokenOpts, true)
	})
}

// TestPaginator_CreateIndexIDOrdering proves pages follow the numeric
// (CreateIndex, ID) iteration order, where 9 sorts before 10 although the
// token "10.a" sorts before "9.a" as a string.
func TestPaginator_CreateIndexIDOrdering(t *testing.T) {
	ci.Parallel(t)

	// (CreateIndex, ID) order, as memdb iterates a "create" index
	ascending := []*mockObject{
		{createIndex: 8, id: "a"},
		{createIndex: 9, id: "a"},
		{createIndex: 10, id: "a"},
		{createIndex: 10, id: "b"},
		{createIndex: 11, id: "a"},
		{createIndex: 12, id: "a"},
	}
	tokenOpts := StructsTokenizerOptions{WithCreateIndex: true, WithID: true}

	t.Run("ascending", func(t *testing.T) {
		requirePagesInOrder(t, ascending, tokenOpts, false)
	})
	t.Run("descending", func(t *testing.T) {
		requirePagesInOrder(t, reversed(ascending), tokenOpts, true)
	})
}

// helpers for pagination tests

// requirePagesInOrder follows NextToken from the first page to the last, two
// objects per page, and requires every object exactly once in iterator order.
func requirePagesInOrder(t *testing.T, mocks []*mockObject, tokenOpts StructsTokenizerOptions, reverse bool) {
	t.Helper()

	tokenizer := StructsTokenizer{opts: tokenOpts}
	want := []string{}
	for _, m := range mocks {
		want = append(want, tokenizer.GetToken(m))
	}

	got := []string{}
	nextToken := ""
	for page := 1; page <= len(mocks); page++ {
		// a fresh iterator per page, as each List RPC gets from the state store
		iter := newTestIteratorWithMocks(mocks)
		opts := structs.QueryOptions{PerPage: 2, NextToken: nextToken, Reverse: reverse}

		paginator, err := NewPaginator(iter, NewStructsTokenizer(iter, tokenOpts), nil, opts,
			func(raw interface{}) error {
				got = append(got, tokenizer.GetToken(raw))
				return nil
			},
		)
		require.NoError(t, err)

		nextToken, err = paginator.Page()
		require.NoError(t, err)
		if nextToken == "" {
			require.Equal(t, want, got, "every object exactly once, in iterator order")
			return
		}
	}
	t.Fatalf("pagination did not finish after %d pages, returned %v", len(mocks), got)
}

// reversed returns a copy of mocks in the order a reverse iterator returns them.
func reversed(mocks []*mockObject) []*mockObject {
	out := slices.Clone(mocks)
	slices.Reverse(out)
	return out
}

// implements Iterator interface
type testResultIterator struct {
	results chan interface{}
}

func (i testResultIterator) Next() interface{} {
	select {
	case raw := <-i.results:
		if raw == nil {
			return nil
		}

		m := raw.(*mockObject)
		return m
	default:
		return nil
	}
}

type mockObject struct {
	id          string
	namespace   string
	createIndex uint64
}

func (m *mockObject) GetNamespace() string {
	return m.namespace
}

func (m *mockObject) GetCreateIndex() uint64 {
	return m.createIndex
}

func (m *mockObject) GetID() string {
	return m.id
}

func newTestIterator(ids []string) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, id := range ids {
		iter.results <- &mockObject{id: id}
	}
	return iter
}

func newTestIteratorWithMocks(mocks []*mockObject) testResultIterator {
	iter := testResultIterator{results: make(chan interface{}, 20)}
	for _, m := range mocks {
		iter.results <- m
	}
	return iter
}

// implements Tokenizer interface
type testTokenizer struct{}

func (t testTokenizer) GetToken(raw interface{}) string {
	return raw.(*mockObject).id
}
