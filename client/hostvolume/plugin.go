// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

// Package hostvolume discovers and runs dynamic host volume plugins, the
// operator-installed executables found in the client's host volume plugin
// directory.
//
// Wander policy for everything the recorded public evidence leaves open is
// documented in docs/compatibility/dynamic-host-volumes-provenance.md.
package hostvolume

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	goversion "github.com/hashicorp/go-version"
)

const (
	// OperationFingerprint is the first argument a plugin receives to
	// report its version.
	OperationFingerprint = "fingerprint"

	// EnvOperation names the protocol variable carrying the operation.
	EnvOperation = "DHV_OPERATION"

	// DefaultFingerprintTimeout is the recorded fingerprint time limit.
	DefaultFingerprintTimeout = 5 * time.Second

	// DefaultStdoutLimit and DefaultStderrLimit bound captured output in
	// bytes. Output beyond the limit kills the plugin.
	DefaultStdoutLimit = 1024
	DefaultStderrLimit = 1024

	// MaxCandidates caps how many plugins one discovery pass returns.
	MaxCandidates = 32

	// MaxIDLen and MaxVersionLen bound advertised strings.
	MaxIDLen      = 64
	MaxVersionLen = 64

	// MaxDiagDetail bounds the sanitised plugin output kept in a diagnostic.
	MaxDiagDetail = 256

	// BuiltinMkdirID is the identity reserved for the built-in directory
	// plugin; an external file with this name is never run.
	BuiltinMkdirID = "mkdir"

	// pluginPath is the fixed PATH given to plugins so that script plugins
	// can find basic tools. It is the only non-protocol variable.
	pluginPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	waitDelay = time.Second
)

// Reason codes for diagnostics. invalid_output means stdout was not a single
// JSON object (or its version was not a string); invalid_version means the
// object carried a missing, empty, over-long, unsafe or non-go-version value.
const (
	ReasonListFailed     = "list_failed"
	ReasonTooManyPlugins = "too_many_plugins"
	ReasonNotExecutable  = "not_executable"
	ReasonInvalidID      = "invalid_id"
	ReasonReservedID     = "reserved_id"
	ReasonStartFailed    = "start_failed"
	ReasonTimeout        = "timeout"
	ReasonCanceled       = "canceled"
	ReasonOutputTooLarge = "output_too_large"
	ReasonNonZeroExit    = "nonzero_exit"
	ReasonInvalidOutput  = "invalid_output"
	ReasonInvalidVersion = "invalid_version"
	ReasonUnsupportedOS  = "unsupported_platform"
)

// Candidate is an executable regular file found directly in the plugin
// directory.
type Candidate struct {
	ID   string
	Path string
}

// Diagnostic explains why a directory entry or plugin was excluded. It never
// holds more than MaxDiagDetail bytes of plugin output and that output has
// control characters removed.
type Diagnostic struct {
	ID     string
	Reason string
	Detail string
}

func (d Diagnostic) String() string {
	if d.Detail == "" {
		return d.ID + ": " + d.Reason
	}
	return d.ID + ": " + d.Reason + ": " + d.Detail
}

// Lister lists a directory, like os.ReadDir.
type Lister func(string) ([]os.DirEntry, error)

// Discover lists dir (top level only) and returns executable regular files
// sorted by ID. An empty or missing dir yields no candidates and no
// diagnostics. Only entries whose own type is regular qualify (Lstat
// semantics), so symlinks, directories, sockets and FIFOs are skipped
// silently, as are names starting with a dot. A time-of-check to time-of-use
// swap of a file is accepted: the directory is operator-owned.
func Discover(dir string) ([]Candidate, []Diagnostic) {
	return DiscoverWith(dir, os.ReadDir)
}

// DiscoverWith is Discover with an injectable lister.
func DiscoverWith(dir string, list Lister) ([]Candidate, []Diagnostic) {
	if dir == "" {
		return nil, nil
	}
	if !execBitMeaningful {
		return nil, []Diagnostic{{ID: "-", Reason: ReasonUnsupportedOS}}
	}
	entries, err := list(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []Diagnostic{{ID: "-", Reason: ReasonListFailed, Detail: sanitize([]byte(err.Error()))}}
	}

	var cands []Candidate
	var diags []Diagnostic
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.Type().IsRegular() {
			continue
		}
		display := sanitize([]byte(name))
		info, err := entry.Info()
		if err != nil || info.Mode()&0o111 == 0 {
			diags = append(diags, Diagnostic{ID: display, Reason: ReasonNotExecutable})
			continue
		}
		if err := ValidateID(name); err != nil {
			diags = append(diags, Diagnostic{ID: display, Reason: ReasonInvalidID})
			continue
		}
		if name == BuiltinMkdirID {
			diags = append(diags, Diagnostic{ID: name, Reason: ReasonReservedID})
			continue
		}
		cands = append(cands, Candidate{ID: name, Path: filepath.Join(dir, name)})
	}

	sort.Slice(cands, func(i, j int) bool { return cands[i].ID < cands[j].ID })
	sort.SliceStable(diags, func(i, j int) bool { return diags[i].ID < diags[j].ID })
	if len(cands) > MaxCandidates {
		for _, c := range cands[MaxCandidates:] {
			diags = append(diags, Diagnostic{ID: c.ID, Reason: ReasonTooManyPlugins})
		}
		cands = cands[:MaxCandidates]
	}
	return cands, diags
}

// ValidateID checks a plugin ID: 1 to MaxIDLen characters from
// [A-Za-z0-9._-], not starting with a dot.
func ValidateID(id string) error {
	if id == "" || len(id) > MaxIDLen || id[0] == '.' {
		return errors.New("invalid plugin id")
	}
	for i := 0; i < len(id); i++ {
		if !idChar(id[i]) {
			return errors.New("invalid plugin id")
		}
	}
	return nil
}

func idChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '.' || c == '_' || c == '-'
}

// errInvalidOutput and errInvalidVersion distinguish a stdout that is not a
// single JSON object of the expected shape (ReasonInvalidOutput) from an
// object whose version is missing or unacceptable (ReasonInvalidVersion).
var (
	errInvalidOutput  = errors.New("invalid fingerprint output")
	errInvalidVersion = errors.New("invalid version")
)

// ParseVersion validates fingerprint stdout. It must be exactly one JSON
// object, optionally surrounded by whitespace, with a string "version" field.
// Unknown fields are ignored (Wander policy); anything after the object,
// including a second object, is rejected. The version must be non-empty, at
// most MaxVersionLen bytes, made only of [A-Za-z0-9._+-] (safe as a node
// attribute value) and accepted by go-version. It is returned exactly as the
// plugin printed it. A malformed envelope wraps errInvalidOutput, a bad
// version wraps errInvalidVersion.
func ParseVersion(stdout []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return "", errInvalidOutput
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", errInvalidOutput // trailing data
	}
	raw, ok := obj["version"]
	if !ok {
		return "", errInvalidVersion
	}
	var v string
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", errInvalidVersion
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", errInvalidOutput // not a string
	}
	if v == "" || len(v) > MaxVersionLen {
		return "", errInvalidVersion
	}
	for i := 0; i < len(v); i++ {
		if c := v[i]; !idChar(c) && c != '+' {
			return "", errInvalidVersion
		}
	}
	if _, err := goversion.NewVersion(v); err != nil {
		return "", errInvalidVersion
	}
	return v, nil
}

// Limits bounds the captured output of a plugin run, in bytes.
type Limits struct {
	Stdout int
	Stderr int
	// TruncateStdout makes stdout overflow non-fatal: output beyond Stdout
	// is discarded and reported in RunResult.StdoutTruncated, and the plugin
	// keeps running. Stderr overflow still kills. Zero keeps the default.
	TruncateStdout bool
}

// RunResult holds bounded output of a plugin run.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	Overflow bool // output exceeded a limit; the plugin was killed

	// StdoutTruncated is set when Limits.TruncateStdout was in effect and
	// stdout exceeded its limit. The plugin was not killed for it.
	StdoutTruncated bool
}

// runPlugin runs path with an explicit argv and exactly the given
// environment, never through a shell. Output is captured up to limits; if
// either limit is exceeded the process is killed. On Unix the process group
// is killed on cancellation. Later operations (create, delete) reuse this.
func runPlugin(ctx context.Context, path string, args, env []string, limits Limits) (RunResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var out, errb boundedBuffer
	out.limit = limits.Stdout
	if !limits.TruncateStdout {
		out.onOverflow = cancel
	}
	errb.limit, errb.onOverflow = limits.Stderr, cancel

	cmd := exec.CommandContext(ctx, path, args...)
	if env == nil {
		env = []string{} // nil would inherit the agent environment
	}
	cmd.Env = env
	cmd.Dir = filepath.Dir(path)
	cmd.Stdin = nil
	cmd.Stdout = &out
	cmd.Stderr = &errb
	cmd.WaitDelay = waitDelay
	configureProcess(cmd)

	err := cmd.Run()
	// Run has returned, so the copying goroutines are done and the buffers
	// can be read without further synchronisation.
	res := RunResult{
		Stdout:   out.buf.Bytes(),
		Stderr:   errb.buf.Bytes(),
		Overflow: (out.overflow && !limits.TruncateStdout) || errb.overflow,

		StdoutTruncated: out.overflow && limits.TruncateStdout,
	}
	return res, err
}

// boundedBuffer keeps at most limit bytes and discards the rest, always
// reporting a full write so the child is never blocked on a pipe.
type boundedBuffer struct {
	buf        bytes.Buffer
	limit      int
	overflow   bool
	onOverflow func()
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room < 0 {
		room = 0
	}
	if len(p) > room {
		b.buf.Write(p[:room])
		if !b.overflow {
			b.overflow = true
			if b.onOverflow != nil {
				b.onOverflow()
			}
		}
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

// Result is the outcome of fingerprinting one candidate.
type Result struct {
	ID      string
	Version string
	Healthy bool
	Diag    *Diagnostic
}

// Runner fingerprints plugins. Zero fields take their defaults.
type Runner struct {
	Timeout time.Duration
	Limits  Limits
}

// Fingerprint runs the candidate's fingerprint operation: argv is
// ["fingerprint"], the environment is DHV_OPERATION=fingerprint plus a fixed
// PATH, and a healthy plugin exits 0 with a JSON object such as
// {"version": "0.0.1"} on stdout (see ParseVersion).
func (r Runner) Fingerprint(ctx context.Context, c Candidate) Result {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultFingerprintTimeout
	}
	limits := r.Limits
	if limits.Stdout <= 0 {
		limits.Stdout = DefaultStdoutLimit
	}
	if limits.Stderr <= 0 {
		limits.Stderr = DefaultStderrLimit
	}

	fail := func(reason, detail string) Result {
		return Result{ID: c.ID, Diag: &Diagnostic{ID: c.ID, Reason: reason, Detail: detail}}
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	env := []string{EnvOperation + "=" + OperationFingerprint, "PATH=" + pluginPath}
	res, err := runPlugin(tctx, c.Path, []string{OperationFingerprint}, env, limits)

	switch {
	case res.Overflow:
		return fail(ReasonOutputTooLarge, "")
	case ctx.Err() != nil:
		return fail(ReasonCanceled, "")
	case errors.Is(tctx.Err(), context.DeadlineExceeded):
		return fail(ReasonTimeout, "")
	case err != nil:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fail(ReasonNonZeroExit, fmt.Sprintf("exit %d: %s", ee.ExitCode(), sanitize(res.Stderr)))
		}
		return fail(ReasonStartFailed, sanitize([]byte(err.Error())))
	}

	v, verr := ParseVersion(res.Stdout)
	if verr != nil {
		reason := ReasonInvalidVersion
		if errors.Is(verr, errInvalidOutput) {
			reason = ReasonInvalidOutput
		}
		return fail(reason, sanitize(res.Stdout))
	}
	return Result{ID: c.ID, Version: v, Healthy: true}
}

// sanitize returns at most MaxDiagDetail bytes of b with control characters
// and invalid UTF-8 replaced by spaces.
func sanitize(b []byte) string {
	var sb strings.Builder
	for len(b) > 0 && sb.Len() < MaxDiagDetail {
		r, n := utf8.DecodeRune(b)
		b = b[n:]
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			r = ' '
		}
		if sb.Len()+utf8.RuneLen(r) > MaxDiagDetail {
			break
		}
		sb.WriteRune(r)
	}
	return strings.TrimSpace(sb.String())
}
