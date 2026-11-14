// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package paginator

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// Tokenizer is the interface that must be implemented to provide pagination
// tokens to the Paginator.
type Tokenizer interface {
	// GetToken returns the pagination token for the given element.
	GetToken(interface{}) string
}

// Comparer is an optional interface for a Tokenizer whose tokens do not sort
// as plain strings in iteration order. Compare returns -1, 0 or +1 when the
// token of raw sorts before, equal to, or after target. A Tokenizer without it
// is compared with strings.Compare.
type Comparer interface {
	Compare(raw interface{}, target string) int
}

// IDGetter is the interface that must be implemented by structs that need to
// have their ID as part of the pagination token.
type IDGetter interface {
	GetID() string
}

// NamespaceGetter is the interface that must be implemented by structs that
// need to have their Namespace as part of the pagination token.
type NamespaceGetter interface {
	GetNamespace() string
}

// CreateIndexGetter is the interface that must be implemented by structs that
// need to have their CreateIndex as part of the pagination token.
type CreateIndexGetter interface {
	GetCreateIndex() uint64
}

// StructsTokenizerOptions is the configuration provided to a StructsTokenizer.
//
// These are some of the common use cases:
//
// Structs that can be uniquely identified with only its own ID:
//
//	StructsTokenizerOptions {
//	    WithID: true,
//	}
//
// Structs that are only unique within their namespace:
//
//	StructsTokenizerOptions {
//	    WithID:        true,
//	    WithNamespace: true,
//	}
//
// Structs that can be sorted by their create index should also set
// `WithCreateIndex` to `true` along with the other options:
//
//	StructsTokenizerOptions {
//	    WithID:          true,
//	    WithNamespace:   true,
//	    WithCreateIndex: true,
//	}
type StructsTokenizerOptions struct {
	WithCreateIndex bool
	WithNamespace   bool
	WithID          bool
}

// StructsTokenizer is an pagination token generator that can create different
// formats of pagination tokens based on common fields found in the structs
// package.
type StructsTokenizer struct {
	opts StructsTokenizerOptions
}

// NewStructsTokenizer returns a new StructsTokenizer.
func NewStructsTokenizer(_ Iterator, opts StructsTokenizerOptions) StructsTokenizer {
	return StructsTokenizer{
		opts: opts,
	}
}

func (it StructsTokenizer) GetToken(raw interface{}) string {
	if raw == nil {
		return ""
	}

	parts := []string{}

	if it.opts.WithCreateIndex {
		token := raw.(CreateIndexGetter).GetCreateIndex()
		parts = append(parts, fmt.Sprintf("%v", token))
	}

	if it.opts.WithNamespace {
		token := raw.(NamespaceGetter).GetNamespace()
		parts = append(parts, token)
	}

	if it.opts.WithID {
		token := raw.(IDGetter).GetID()
		parts = append(parts, token)
	}

	// Use a character that is not part of validNamespaceName as separator to
	// avoid accidentally generating collisions.
	// For example, namespace `a` and job `b-c` would collide with namespace
	// `a-b` and job `c` into the same token `a-b-c`, since `-` is an allowed
	// character in namespace.
	return strings.Join(parts, ".")
}

// Compare orders the token of raw against target field by field, as memdb
// orders compound indexes. Whole-string order differs: "team-a.j1" sorts
// before "team.j1" because '-' < '.', and "10.x" sorts before "9.x". A target
// not in this tokenizer's format is compared as a plain string.
func (it StructsTokenizer) Compare(raw interface{}, target string) int {
	fields := 0
	for _, with := range []bool{it.opts.WithCreateIndex, it.opts.WithNamespace, it.opts.WithID} {
		if with {
			fields++
		}
	}

	// only the last field, an ID, may contain '.', so SplitN keeps it whole
	segments := strings.SplitN(target, ".", fields)
	if fields == 0 || len(segments) != fields {
		return strings.Compare(it.GetToken(raw), target)
	}

	if it.opts.WithCreateIndex {
		index, err := strconv.ParseUint(segments[0], 10, 64)
		if err != nil {
			return strings.Compare(it.GetToken(raw), target)
		}
		if c := cmp.Compare(raw.(CreateIndexGetter).GetCreateIndex(), index); c != 0 {
			return c
		}
		segments = segments[1:]
	}

	if it.opts.WithNamespace {
		if c := strings.Compare(raw.(NamespaceGetter).GetNamespace(), segments[0]); c != 0 {
			return c
		}
		segments = segments[1:]
	}

	if it.opts.WithID {
		return strings.Compare(raw.(IDGetter).GetID(), segments[0])
	}
	return 0
}
