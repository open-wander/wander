// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

// Package state holds the record the client persists for each dynamic host
// volume on the node. It is a leaf package so that the client state store can
// use the type without importing the code that runs plugins.
package state

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
	"unicode"
	"unicode/utf8"
)

// Kind says how a volume came to be on the node.
type Kind string

const (
	// KindRegistered is an existing external path the operator registered.
	// It is only ever checked for existence, never written to.
	KindRegistered Kind = "registered"

	// KindPlugin is a volume a plugin created, and re-creates on restore.
	KindPlugin Kind = "plugin"
)

// Status is the last observed availability of a volume.
type Status string

const (
	StatusReady       Status = "ready"
	StatusUnavailable Status = "unavailable"
)

// Reason codes for an unavailable volume (Wander policy).
const (
	ReasonMissingPath       = "missing_path"
	ReasonPluginUnavailable = "plugin_unavailable"
	ReasonRestoreFailed     = "restore_failed"
	ReasonPathChanged       = "path_changed"
	ReasonNodeMismatch      = "node_mismatch"
	ReasonNameConflict      = "name_conflict"
	ReasonInvalidRecord     = "invalid_record"
)

const (
	// MaxIDLen bounds the volume ID, namespace, name and node ID.
	MaxIDLen = 256

	// MaxPathLen bounds the volume path.
	MaxPathLen = 4096

	// MaxMessageLen bounds the diagnostic message.
	MaxMessageLen = 256
)

// Record is the persisted client-side state of one dynamic host volume:
// what the volume is, how it is recreated and the last diagnostic. The
// request fields (including Parameters) are the inputs a plugin was given;
// the diagnostic fields never hold a parameter value.
type Record struct {
	VolumeID  string
	Namespace string
	Name      string
	NodeID    string
	NodePool  string

	Kind     Kind
	PluginID string

	CapacityMinBytes int64
	CapacityMaxBytes int64
	Parameters       map[string]string

	// Path is the volume path. For a plugin volume it is the path the
	// plugin returned when the volume was created.
	Path string

	Status    Status
	Reason    string
	Message   string
	CheckedAt time.Time
}

// Copy returns a deep copy.
func (r *Record) Copy() *Record {
	if r == nil {
		return nil
	}
	c := *r
	if r.Parameters != nil {
		c.Parameters = make(map[string]string, len(r.Parameters))
		for k, v := range r.Parameters {
			c.Parameters[k] = v
		}
	}
	return &c
}

// Validate checks a record. Records loaded from disk are untrusted. The
// plugin ID syntax is checked by the caller, which knows the plugin rules.
// Errors never include parameter values.
func (r *Record) Validate() error {
	if err := validToken("volume id", r.VolumeID); err != nil {
		return err
	}
	for i := 0; i < len(r.VolumeID); i++ {
		c := r.VolumeID[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return errors.New("volume id has an invalid character")
		}
	}
	if err := validToken("name", r.Name); err != nil {
		return err
	}
	if err := validToken("node id", r.NodeID); err != nil {
		return err
	}
	if r.Namespace != "" {
		if err := validToken("namespace", r.Namespace); err != nil {
			return err
		}
	}
	if r.NodePool != "" {
		if err := validToken("node pool", r.NodePool); err != nil {
			return err
		}
	}
	switch r.Kind {
	case KindRegistered:
		if r.PluginID != "" {
			return errors.New("a registered volume has no plugin")
		}
	case KindPlugin:
		if r.PluginID == "" {
			return errors.New("plugin id is empty")
		}
	default:
		return fmt.Errorf("unknown kind %q", truncate(string(r.Kind)))
	}
	if r.CapacityMinBytes < 0 || r.CapacityMaxBytes < 0 {
		return errors.New("capacity is negative")
	}
	if err := validPath(r.Path); err != nil {
		return err
	}
	switch r.Status {
	case StatusReady, StatusUnavailable:
	default:
		return fmt.Errorf("unknown status %q", truncate(string(r.Status)))
	}
	if len(r.Reason) > MaxIDLen || len(r.Message) > MaxMessageLen {
		return errors.New("diagnostic is too long")
	}
	return nil
}

func truncate(s string) string {
	if len(s) > 32 {
		return s[:32]
	}
	return s
}

// validToken checks a non-empty bounded printable string.
func validToken(what, s string) error {
	switch {
	case s == "":
		return fmt.Errorf("%s is empty", what)
	case len(s) > MaxIDLen:
		return fmt.Errorf("%s is too long", what)
	case !utf8.ValidString(s):
		return fmt.Errorf("%s is not valid UTF-8", what)
	}
	for _, r := range s {
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			return fmt.Errorf("%s has a non-printable character", what)
		}
	}
	return nil
}

// validPath requires a bounded, printable, absolute, clean path that is not
// a filesystem root.
func validPath(p string) error {
	switch {
	case p == "":
		return errors.New("path is empty")
	case len(p) > MaxPathLen:
		return errors.New("path is too long")
	case !utf8.ValidString(p):
		return errors.New("path is not valid UTF-8")
	}
	for _, r := range p {
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			return errors.New("path has a non-printable character")
		}
	}
	switch {
	case !filepath.IsAbs(p):
		return errors.New("path is not absolute")
	case filepath.Clean(p) != p:
		return errors.New("path is not clean")
	case filepath.Dir(p) == p:
		return errors.New("path is a filesystem root")
	}
	return nil
}
