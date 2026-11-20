// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package hostvolume

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Operations a plugin is called with besides fingerprint.
const (
	OperationCreate = "create"
	OperationDelete = "delete"
)

// Environment variable names of the create and delete protocol, as recorded
// in DHV-B-084 to DHV-B-095. EnvOperation is declared in plugin.go.
const (
	EnvVolumesDir       = "DHV_VOLUMES_DIR"
	EnvPluginDir        = "DHV_PLUGIN_DIR"
	EnvNamespace        = "DHV_NAMESPACE"
	EnvVolumeName       = "DHV_VOLUME_NAME"
	EnvVolumeID         = "DHV_VOLUME_ID"
	EnvNodeID           = "DHV_NODE_ID"
	EnvNodePool         = "DHV_NODE_POOL"
	EnvCapacityMinBytes = "DHV_CAPACITY_MIN_BYTES"
	EnvCapacityMaxBytes = "DHV_CAPACITY_MAX_BYTES"
	EnvParameters       = "DHV_PARAMETERS"
	EnvCreatedPath      = "DHV_CREATED_PATH"
)

const (
	// DefaultCreateTimeout and DefaultDeleteTimeout are the recorded time
	// limits (DHV-B-081, DHV-B-082). They start after the per-volume lock
	// is held.
	DefaultCreateTimeout = 60 * time.Second
	DefaultDeleteTimeout = 60 * time.Second

	// CreateStdoutLimit bounds create stdout. It must exceed MaxPathLen plus
	// the JSON envelope, with room for escaping. Exceeding it kills the
	// plugin. The recorded evidence does not state output limits
	// (DHV-B-100), so this is Wander policy.
	CreateStdoutLimit = 16 * 1024

	// DeleteStdoutCapture bounds the delete stdout kept for reading the
	// error object. Delete stdout is otherwise discarded, and exceeding the
	// bound truncates without killing the plugin.
	DeleteStdoutCapture = 4 * 1024

	// OpStderrLimit bounds stderr for create and delete. Exceeding it kills
	// the plugin.
	OpStderrLimit = 4 * 1024

	// MaxPathLen bounds a path returned by create or passed to delete.
	MaxPathLen = 4096

	// MaxParametersBytes bounds the encoded DHV_PARAMETERS text.
	MaxParametersBytes = 64 * 1024

	// MaxErrorMessage bounds a plugin error message surfaced to the caller.
	MaxErrorMessage = MaxDiagDetail

	redacted = "[redacted]"
)

// Reason codes added for create and delete. The others are shared with
// fingerprint in plugin.go.
const (
	ReasonInvalidInput = "invalid_input"
	ReasonInvalidPath  = "invalid_path"

	// ReasonVolumeKindConflict means a registration used the ID of a
	// plugin-created volume. ReasonRegistrationMismatch means a repeat
	// registration differs in namespace, name or path.
	ReasonVolumeKindConflict   = "volume_kind_conflict"
	ReasonRegistrationMismatch = "registration_mismatch"
)

// OpRequest holds the values for one create or delete call. Zero capacities
// mean unset. Parameter values are sensitive: they are never put in errors.
type OpRequest struct {
	VolumeID         string
	Namespace        string
	Name             string
	NodeID           string
	NodePool         string
	CapacityMinBytes int64
	CapacityMaxBytes int64
	Parameters       map[string]string
}

// CreateResult is a validated create response.
type CreateResult struct {
	Path  string
	Bytes int64
}

// OpError is a classified failure of a create or delete call. Reason is one
// of the stable Reason constants. Message is sanitised plugin or Wander
// text; it never holds a parameter value.
type OpError struct {
	Op      string
	Reason  string
	Message string
}

func (e *OpError) Error() string {
	s := "host volume plugin " + e.Op + ": " + e.Reason
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

func opErr(op, reason, msg string) *OpError {
	return &OpError{Op: op, Reason: reason, Message: msg}
}

// Ops runs create and delete operations. The zero Timeout selects the
// recorded 60 seconds. An Ops holds the per-volume lock table, so it must be
// shared by every caller in the client process and not copied after use.
type Ops struct {
	PluginDir  string
	VolumesDir string
	Timeout    time.Duration

	locks lockTable
}

// Create runs the candidate's create operation and returns the validated
// result. On every failure the result is the zero value. A failed create
// is not retried here (DHV-B-101); retrying with the same request calls the
// plugin again with identical inputs (DHV-B-083).
func (o *Ops) Create(ctx context.Context, c Candidate, req OpRequest) (CreateResult, error) {
	params, err := validateRequest(OperationCreate, c, req, "")
	if err != nil {
		return CreateResult{}, err
	}
	env := o.buildEnv(OperationCreate, req, params, "")
	res, oerr := o.run(ctx, OperationCreate, c, req, env, Limits{Stdout: CreateStdoutLimit, Stderr: OpStderrLimit})
	if oerr != nil {
		return CreateResult{}, oerr
	}
	out, oerr := parseCreateOutput(res.Stdout)
	if oerr != nil {
		return CreateResult{}, oerr
	}
	return out, nil
}

// Delete runs the candidate's delete operation. createdPath is the path the
// create call returned. Stdout is discarded; on exit 0 delete succeeds
// whatever it printed, so deleting an already deleted volume succeeds when
// the plugin does.
func (o *Ops) Delete(ctx context.Context, c Candidate, req OpRequest, createdPath string) error {
	params, err := validateRequest(OperationDelete, c, req, createdPath)
	if err != nil {
		return err
	}
	env := o.buildEnv(OperationDelete, req, params, createdPath)
	limits := Limits{Stdout: DeleteStdoutCapture, Stderr: OpStderrLimit, TruncateStdout: true}
	if _, oerr := o.run(ctx, OperationDelete, c, req, env, limits); oerr != nil {
		return oerr
	}
	return nil
}

// timeout returns the time limit of an operation: the configured one, or the
// recorded default of the operation when none is set.
func (o *Ops) timeout(op string) time.Duration {
	switch {
	case o.Timeout > 0:
		return o.Timeout
	case op == OperationDelete:
		return DefaultDeleteTimeout
	}
	return DefaultCreateTimeout
}

// run takes the per-volume lock, starts the operation deadline and runs the
// plugin. It returns a classified error for every outcome except exit 0.
func (o *Ops) run(ctx context.Context, op string, c Candidate, req OpRequest, env []string, limits Limits) (RunResult, *OpError) {
	if err := o.locks.acquire(ctx, req.VolumeID); err != nil {
		return RunResult{}, opErr(op, ReasonCanceled, "")
	}
	defer o.locks.release(req.VolumeID)
	if ctx.Err() != nil {
		return RunResult{}, opErr(op, ReasonCanceled, "")
	}

	tctx, cancel := context.WithTimeout(ctx, o.timeout(op))
	defer cancel()
	res, err := runPlugin(tctx, c.Path, []string{op}, env, limits)

	switch {
	case res.Overflow:
		return RunResult{}, opErr(op, ReasonOutputTooLarge, "")
	case ctx.Err() != nil:
		return RunResult{}, opErr(op, ReasonCanceled, "")
	case errors.Is(tctx.Err(), context.DeadlineExceeded):
		return RunResult{}, opErr(op, ReasonTimeout, "")
	case err != nil:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := fmt.Sprintf("exit %d", ee.ExitCode())
			if m := parsePluginError(res.Stdout, res.StdoutTruncated, req.Parameters); m != "" {
				msg = m
			}
			return RunResult{}, opErr(op, ReasonNonZeroExit, msg)
		}
		return RunResult{}, opErr(op, ReasonStartFailed, sanitize([]byte(err.Error())))
	}
	return res, nil
}

// validateRequest rejects bad input before any lock or process is used and
// returns the encoded parameters. Error text never includes a value.
func validateRequest(op string, c Candidate, req OpRequest, createdPath string) (string, error) {
	if err := ValidateID(c.ID); err != nil {
		return "", opErr(op, ReasonInvalidInput, "invalid plugin id")
	}
	if c.ID == BuiltinMkdirID {
		return "", opErr(op, ReasonReservedID, "")
	}
	if c.Path == "" || strings.IndexByte(c.Path, 0) >= 0 {
		return "", opErr(op, ReasonInvalidInput, "invalid plugin path")
	}
	params, oerr := validateFields(op, req)
	if oerr != nil {
		return "", oerr
	}
	if op == OperationDelete {
		if err := validatePath(createdPath); err != nil {
			return "", opErr(op, ReasonInvalidInput, "created path: "+err.Error())
		}
	}
	return params, nil
}

// validateFields holds the request checks shared with the built-in plugin
// and returns the encoded parameters.
func validateFields(op string, req OpRequest) (string, *OpError) {
	if req.VolumeID == "" {
		return "", opErr(op, ReasonInvalidInput, "volume id is empty")
	}
	fields := []struct{ name, v string }{
		{"volume id", req.VolumeID}, {"namespace", req.Namespace}, {"volume name", req.Name},
		{"node id", req.NodeID}, {"node pool", req.NodePool},
	}
	for _, f := range fields {
		if strings.IndexByte(f.v, 0) >= 0 {
			return "", opErr(op, ReasonInvalidInput, f.name+" contains a NUL byte")
		}
	}
	if req.CapacityMinBytes < 0 || req.CapacityMaxBytes < 0 {
		return "", opErr(op, ReasonInvalidInput, "capacity is negative")
	}
	params, perr := encodeParameters(req.Parameters)
	if perr != nil {
		return "", opErr(op, ReasonInvalidInput, perr.Error())
	}
	return params, nil
}

// formatCapacity renders a capacity for DHV_CAPACITY_*_BYTES. Owner decision
// 1 (DHV-B-092, DHV-B-093): base-10 integer, and unset (0) is "0".
func formatCapacity(n int64) string {
	return strconv.FormatInt(n, 10)
}

// encodeParameters renders DHV_PARAMETERS. Owner decision 2 (DHV-B-094): no
// parameters (nil or empty) is exactly "{}". Otherwise the text is compact
// JSON with sorted keys and without HTML escaping, so equal inputs give
// byte-identical text. Errors never include a key or value.
func encodeParameters(p map[string]string) (string, error) {
	if len(p) == 0 {
		// The JSON text for "no parameters", as observed (see the provenance
		// record). A plugin must accept it; "{}" would be a different input.
		return "null", nil
	}
	// json would silently replace invalid UTF-8 with U+FFFD, so check.
	for k, v := range p {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			return "", errors.New("parameters contain invalid UTF-8")
		}
		if strings.IndexByte(k, 0) >= 0 || strings.IndexByte(v, 0) >= 0 {
			return "", errors.New("parameters contain a NUL byte")
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return "", errors.New("parameters cannot be encoded")
	}
	s := strings.TrimSuffix(buf.String(), "\n")
	if len(s) > MaxParametersBytes {
		return "", errors.New("parameters are too large")
	}
	return s, nil
}

// buildEnv returns the complete child environment. Owner decision 3
// (DHV-B-084 to DHV-B-095): every documented variable for the operation is
// always set, an empty value is the empty string. Delete has
// DHV_CREATED_PATH and no capacity variables. PATH is the fixed value used
// for fingerprint. Nothing is inherited from the agent.
func (o *Ops) buildEnv(op string, req OpRequest, params, createdPath string) []string {
	env := make([]string, 0, 13)
	env = append(env, EnvOperation+"="+op)
	if op == OperationDelete {
		env = append(env, EnvCreatedPath+"="+createdPath)
	}
	env = append(env,
		EnvVolumesDir+"="+o.VolumesDir,
		EnvPluginDir+"="+o.PluginDir,
		EnvNamespace+"="+req.Namespace,
		EnvVolumeName+"="+req.Name,
		EnvVolumeID+"="+req.VolumeID,
		EnvNodeID+"="+req.NodeID,
		EnvNodePool+"="+req.NodePool,
	)
	if op == OperationCreate {
		env = append(env,
			EnvCapacityMinBytes+"="+formatCapacity(req.CapacityMinBytes),
			EnvCapacityMaxBytes+"="+formatCapacity(req.CapacityMaxBytes),
		)
	}
	return append(env, EnvParameters+"="+params, "PATH="+pluginPath)
}

// parseCreateOutput validates create stdout from a plugin that exited 0.
// Owner decisions 4 and 5 (DHV-B-081, DHV-B-096): the output is one JSON
// object; path is required and must be a string; bytes is optional, missing
// is 0, and when present must be a non-negative integer written without
// fraction, exponent or sign. Unknown fields are ignored (an error field
// included), data after the object is rejected, and for duplicate keys the
// last one wins.
func parseCreateOutput(stdout []byte) (CreateResult, *OpError) {
	const op = OperationCreate
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return CreateResult{}, opErr(op, ReasonInvalidOutput, "output is not a JSON object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return CreateResult{}, opErr(op, ReasonInvalidOutput, "unexpected data after the JSON object")
	}

	rawPath, ok := obj["path"]
	if !ok || bytes.Equal(rawPath, []byte("null")) {
		return CreateResult{}, opErr(op, ReasonInvalidOutput, "path is missing")
	}
	var path string
	if err := json.Unmarshal(rawPath, &path); err != nil {
		return CreateResult{}, opErr(op, ReasonInvalidOutput, "path is not a string")
	}
	if err := validatePath(path); err != nil {
		return CreateResult{}, opErr(op, ReasonInvalidPath, err.Error())
	}

	var n int64
	if raw, ok := obj["bytes"]; ok {
		v, err := parseBytes(raw)
		if err != nil {
			return CreateResult{}, opErr(op, ReasonInvalidOutput, "bytes is not a non-negative integer")
		}
		n = v
	}
	return CreateResult{Path: path, Bytes: n}, nil
}

// parseBytes accepts only the JSON text 0 or a digit string without a
// leading zero that fits in int64.
func parseBytes(raw json.RawMessage) (int64, error) {
	s := string(raw)
	if s == "" || len(s) > 19 || (len(s) > 1 && s[0] == '0') {
		return 0, errors.New("invalid")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("invalid")
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

// parsePluginError returns the sanitised error message from the stdout of a
// plugin that exited non-zero, or "". Owner decision 5 (DHV-B-096,
// DHV-B-097): it is only called after a non-zero exit. Stdout must be one
// JSON object, complete (not truncated), with a string error field.
// Parameter values are replaced before the text is cut to its bound so that
// a cut value cannot leak a prefix.
func parsePluginError(stdout []byte, truncated bool, params map[string]string) string {
	if truncated {
		return ""
	}
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return ""
	}
	if _, err := dec.Token(); err != io.EOF {
		return ""
	}
	var msg string
	if err := json.Unmarshal(obj["error"], &msg); err != nil {
		return ""
	}
	return sanitize([]byte(redact(msg, params)))
}

// redact replaces every non-empty parameter value in s, longest first.
func redact(s string, params map[string]string) string {
	vals := make([]string, 0, len(params))
	for _, v := range params {
		if v != "" {
			vals = append(vals, v)
		}
	}
	// longest first, so a value that contains another is replaced whole
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && len(vals[j]) > len(vals[j-1]); j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	for _, v := range vals {
		s = strings.ReplaceAll(s, v, redacted)
	}
	return s
}

// validatePath checks a path returned by create or passed to delete: not
// empty, at most MaxPathLen bytes, valid UTF-8 without NUL, control or
// non-printable characters (U+FFFD included), absolute, equal to its
// cleaned form (which rejects "..", "//" and a trailing slash) and not a
// filesystem root. It need not lie under the volumes directory.
func validatePath(p string) error {
	switch {
	case p == "":
		return errors.New("path is empty")
	case len(p) > MaxPathLen:
		return errors.New("path is too long")
	case !utf8.ValidString(p):
		return errors.New("path is not valid UTF-8")
	}
	for _, r := range p {
		if r == utf8.RuneError || r < 0x20 || r == 0x7f || !unicode.IsPrint(r) {
			return errors.New("path contains a control or non-printable character")
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

// lockTable serialises operations per volume ID within the process. Each ID
// has a one-slot channel so that waiting honours a context, and a reference
// count so entries are removed when no holder or waiter remains. The zero
// value is ready to use.
type lockTable struct {
	mu sync.Mutex
	m  map[string]*lockEntry
}

type lockEntry struct {
	ch   chan struct{}
	refs int
}

// acquire waits for the lock on id or for ctx. On error nothing is held.
func (t *lockTable) acquire(ctx context.Context, id string) error {
	t.mu.Lock()
	if t.m == nil {
		t.m = make(map[string]*lockEntry)
	}
	e := t.m[id]
	if e == nil {
		e = &lockEntry{ch: make(chan struct{}, 1)}
		t.m[id] = e
	}
	e.refs++
	t.mu.Unlock() // never wait on the channel while holding mu

	select {
	case e.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		t.unref(id, e)
		return ctx.Err()
	}
}

// release frees a lock taken by a successful acquire.
func (t *lockTable) release(id string) {
	t.mu.Lock()
	e := t.m[id]
	t.mu.Unlock()
	<-e.ch
	t.unref(id, e)
}

func (t *lockTable) unref(id string, e *lockEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e.refs--; e.refs == 0 {
		delete(t.m, id)
	}
}

// len and refs are for tests: the number of entries and the holders plus
// waiters for one ID.
func (t *lockTable) len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.m)
}

func (t *lockTable) refs(id string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e := t.m[id]; e != nil {
		return e.refs
	}
	return 0
}
