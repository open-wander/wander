// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostvolume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

const secretValue = "s3cr3t-synthetic-value-7f3a"

type opEnv struct {
	ops     *Ops
	plugins string
	volumes string
	scratch string // test-owned files, outside the plugin directory
}

func newOpEnv(t *testing.T) opEnv {
	t.Helper()
	e := opEnv{plugins: t.TempDir(), volumes: t.TempDir(), scratch: t.TempDir()}
	e.ops = &Ops{PluginDir: e.plugins, VolumesDir: e.volumes}
	return e
}

func (e opEnv) plugin(t *testing.T, body string) Candidate {
	t.Helper()
	p := writePlugin(t, e.plugins, "plug", body, 0o755)
	return Candidate{ID: "plug", Path: p}
}

func baseReq() OpRequest {
	return OpRequest{
		VolumeID: "vol-1", Namespace: "ns", Name: "data", NodeID: "node-1", NodePool: "pool",
		CapacityMinBytes: 100, CapacityMaxBytes: 200,
		Parameters: map[string]string{"b": "2", "a": "1"},
	}
}

// eventually polls cond until it holds or the limit passes. It never sleeps
// a fixed time: it returns as soon as cond is true.
func eventually(t *testing.T, cond func() bool, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func asOpError(t *testing.T, err error, op, reason string) *OpError {
	t.Helper()
	var oe *OpError
	must.True(t, errors.As(err, &oe), must.Sprintf("not an OpError: %v", err))
	must.Eq(t, op, oe.Op)
	must.Eq(t, reason, oe.Reason, must.Sprintf("message: %q", oe.Message))
	return oe
}

// dumpBody writes argv and the full environment of the plugin to path.
func dumpBody(path string) string {
	return `{ for a in "$@"; do echo "arg=$a"; done; env; } > '` + path + `'`
}

// readDump returns the argv lines and the environment as a map, dropping the
// variables the shell sets for itself.
func readDump(t *testing.T, path string) (args []string, env map[string]string) {
	t.Helper()
	b, err := os.ReadFile(path)
	must.NoError(t, err)
	env = map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if a, ok := strings.CutPrefix(line, "arg="); ok {
			args = append(args, a)
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		switch k {
		case "PWD", "SHLVL", "_", "OLDPWD":
		default:
			env[k] = v
		}
	}
	return args, env
}

func TestOps_Create_ArgvAndEnv(t *testing.T) {
	t.Setenv("DHV_TEST_SENTINEL", "leak")
	e := newOpEnv(t)
	dump := filepath.Join(e.scratch, "dump")
	c := e.plugin(t, dumpBody(dump)+`; echo '{"path":"/srv/v","bytes":5}'`)

	t.Run("full request", func(t *testing.T) {
		res, err := e.ops.Create(context.Background(), c, baseReq())
		must.NoError(t, err)
		must.Eq(t, CreateResult{Path: "/srv/v", Bytes: 5}, res)
		args, env := readDump(t, dump)
		must.Eq(t, []string{"create"}, args)
		must.Eq(t, map[string]string{
			"DHV_OPERATION":          "create",
			"DHV_VOLUMES_DIR":        e.volumes,
			"DHV_PLUGIN_DIR":         e.plugins,
			"DHV_NAMESPACE":          "ns",
			"DHV_VOLUME_NAME":        "data",
			"DHV_VOLUME_ID":          "vol-1",
			"DHV_NODE_ID":            "node-1",
			"DHV_NODE_POOL":          "pool",
			"DHV_CAPACITY_MIN_BYTES": "100",
			"DHV_CAPACITY_MAX_BYTES": "200",
			"DHV_PARAMETERS":         `{"a":"1","b":"2"}`,
			"PATH":                   pluginPath,
		}, env)
	})

	// Owner decisions 1, 2 and 3: unset capacity is 0, no parameters is
	// {}, and an empty value is set to the empty string, not omitted.
	t.Run("unset and empty values", func(t *testing.T) {
		req := OpRequest{VolumeID: "vol-1", Namespace: "ns", Name: "data", NodeID: "node-1"}
		_, err := e.ops.Create(context.Background(), c, req)
		must.NoError(t, err)
		_, env := readDump(t, dump)
		must.Eq(t, "", env["DHV_NODE_POOL"])
		must.Eq(t, "0", env["DHV_CAPACITY_MIN_BYTES"])
		must.Eq(t, "0", env["DHV_CAPACITY_MAX_BYTES"])
		must.Eq(t, "{}", env["DHV_PARAMETERS"])
		must.MapLen(t, 12, env)
		for _, k := range []string{"DHV_NODE_POOL", "DHV_NAMESPACE", "DHV_PARAMETERS"} {
			_, ok := env[k]
			must.True(t, ok, must.Sprint(k))
		}
	})
}

func TestOps_Delete_ArgvAndEnv(t *testing.T) {
	t.Setenv("DHV_TEST_SENTINEL", "leak")
	e := newOpEnv(t)
	dump := filepath.Join(e.scratch, "dump")
	c := e.plugin(t, dumpBody(dump))

	must.NoError(t, e.ops.Delete(context.Background(), c, baseReq(), "/srv/v"))
	args, env := readDump(t, dump)
	must.Eq(t, []string{"delete"}, args)
	must.Eq(t, map[string]string{
		"DHV_OPERATION":    "delete",
		"DHV_CREATED_PATH": "/srv/v",
		"DHV_VOLUMES_DIR":  e.volumes,
		"DHV_PLUGIN_DIR":   e.plugins,
		"DHV_NAMESPACE":    "ns",
		"DHV_VOLUME_NAME":  "data",
		"DHV_VOLUME_ID":    "vol-1",
		"DHV_NODE_ID":      "node-1",
		"DHV_NODE_POOL":    "pool",
		"DHV_PARAMETERS":   `{"a":"1","b":"2"}`,
		"PATH":             pluginPath,
	}, env)

	// empty values stay set on delete too
	req := OpRequest{VolumeID: "vol-1"}
	must.NoError(t, e.ops.Delete(context.Background(), c, req, "/srv/v"))
	_, env = readDump(t, dump)
	must.Eq(t, "{}", env["DHV_PARAMETERS"])
	must.MapLen(t, 11, env)
}

func TestEncodings(t *testing.T) {
	ci.Parallel(t)
	for in, want := range map[int64]string{0: "0", 1: "1", 1 << 40: "1099511627776"} {
		must.Eq(t, want, formatCapacity(in))
	}

	cases := []struct {
		name string
		in   map[string]string
		want string
	}{
		{"nil", nil, "{}"},
		{"empty", map[string]string{}, "{}"},
		{"sorted", map[string]string{"z": "1", "a": "2", "m": "3"}, `{"a":"2","m":"3","z":"1"}`},
		{"no html escaping", map[string]string{"k": "<a&b>"}, `{"k":"<a&b>"}`},
		{"empty value", map[string]string{"k": ""}, `{"k":""}`},
		{"quote", map[string]string{"k": `a"b`}, `{"k":"a\"b"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encodeParameters(tc.in)
			must.NoError(t, err)
			must.Eq(t, tc.want, got)
		})
	}

	_, err := encodeParameters(map[string]string{"k": strings.Repeat("x", MaxParametersBytes)})
	must.Error(t, err)
}

func TestParseCreateOutput(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name   string
		out    string
		want   CreateResult
		reason string
	}{
		{"path and bytes", `{"path":"/a/b","bytes":42}`, CreateResult{"/a/b", 42}, ""},
		{"bytes missing is 0", `{"path":"/a/b"}`, CreateResult{"/a/b", 0}, ""},
		{"bytes zero", `{"path":"/a/b","bytes":0}`, CreateResult{"/a/b", 0}, ""},
		{"max int64", `{"path":"/a","bytes":9223372036854775807}`, CreateResult{"/a", 9223372036854775807}, ""},
		{"unknown fields ignored", `{"x":[1,{"y":2}],"path":"/a","bytes":1}`, CreateResult{"/a", 1}, ""},
		{"whitespace around", " \n{\"path\":\"/a\"}\n\n", CreateResult{"/a", 0}, ""},
		{"last duplicate wins", `{"path":"/a","path":"/b"}`, CreateResult{"/b", 0}, ""},
		{"error field ignored when path valid", `{"path":"/a","error":"x"}`, CreateResult{"/a", 0}, ""},

		{"error only is invalid", `{"error":"boom"}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes only", `{"bytes":3}`, CreateResult{}, ReasonInvalidOutput},
		{"path null", `{"path":null}`, CreateResult{}, ReasonInvalidOutput},
		{"path number", `{"path":5}`, CreateResult{}, ReasonInvalidOutput},
		{"path array", `{"path":["/a"]}`, CreateResult{}, ReasonInvalidOutput},
		{"empty stdout", ``, CreateResult{}, ReasonInvalidOutput},
		{"not json", `/a/b`, CreateResult{}, ReasonInvalidOutput},
		{"array", `[{"path":"/a"}]`, CreateResult{}, ReasonInvalidOutput},
		{"null", `null`, CreateResult{}, ReasonInvalidOutput},
		{"truncated json", `{"path":"/a"`, CreateResult{}, ReasonInvalidOutput},
		{"trailing data", `{"path":"/a"} x`, CreateResult{}, ReasonInvalidOutput},
		{"second object", `{"path":"/a"}{"path":"/b"}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes fraction", `{"path":"/a","bytes":1.5}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes 1.0", `{"path":"/a","bytes":1.0}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes exponent", `{"path":"/a","bytes":1e3}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes string", `{"path":"/a","bytes":"5"}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes negative", `{"path":"/a","bytes":-1}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes negative zero", `{"path":"/a","bytes":-0}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes leading zero", `{"path":"/a","bytes":01}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes overflow", `{"path":"/a","bytes":9223372036854775808}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes huge", `{"path":"/a","bytes":99999999999999999999999}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes null", `{"path":"/a","bytes":null}`, CreateResult{}, ReasonInvalidOutput},
		{"bytes bool", `{"path":"/a","bytes":true}`, CreateResult{}, ReasonInvalidOutput},

		{"path empty", `{"path":""}`, CreateResult{}, ReasonInvalidPath},
		{"path relative", `{"path":"a/b"}`, CreateResult{}, ReasonInvalidPath},
		{"path dot relative", `{"path":"./a"}`, CreateResult{}, ReasonInvalidPath},
		{"path traversal", `{"path":"/a/../b"}`, CreateResult{}, ReasonInvalidPath},
		{"path dotdot only", `{"path":"/.."}`, CreateResult{}, ReasonInvalidPath},
		{"path trailing slash", `{"path":"/a/"}`, CreateResult{}, ReasonInvalidPath},
		{"path double slash", `{"path":"/a//b"}`, CreateResult{}, ReasonInvalidPath},
		{"path dot segment", `{"path":"/a/./b"}`, CreateResult{}, ReasonInvalidPath},
		{"path root", `{"path":"/"}`, CreateResult{}, ReasonInvalidPath},
		{"path NUL", `{"path":"/a\u0000b"}`, CreateResult{}, ReasonInvalidPath},
		{"path newline", `{"path":"/a\nb"}`, CreateResult{}, ReasonInvalidPath},
		{"path escape char", `{"path":"/a\u001bb"}`, CreateResult{}, ReasonInvalidPath},
		{"path DEL", `{"path":"/a\u007fb"}`, CreateResult{}, ReasonInvalidPath},
		{"path replacement char", `{"path":"/a�b"}`, CreateResult{}, ReasonInvalidPath},
		{"path too long", `{"path":"/` + strings.Repeat("a", MaxPathLen) + `"}`, CreateResult{}, ReasonInvalidPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, oe := parseCreateOutput([]byte(tc.out))
			if tc.reason == "" {
				must.Nil(t, oe)
				must.Eq(t, tc.want, got)
				return
			}
			must.NotNil(t, oe)
			must.Eq(t, tc.reason, oe.Reason)
			must.Eq(t, CreateResult{}, got)
		})
	}

	// the longest allowed path fits in the stdout limit with its envelope
	long := "/" + strings.Repeat("a", MaxPathLen-1)
	got, oe := parseCreateOutput([]byte(`{"path":"` + long + `","bytes":1}`))
	must.Nil(t, oe)
	must.Eq(t, long, got.Path)
	must.Greater(t, MaxPathLen+64, CreateStdoutLimit)
}

// TestOps_Create_Classification runs real plugins through Create for each
// failure class and checks the reason and that no result is returned.
func TestOps_Create_Classification(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{"malformed", `echo not-json`, ReasonInvalidOutput},
		{"trailing data", `echo '{"path":"/a"} junk'`, ReasonInvalidOutput},
		{"error object on exit 0", `echo '{"error":"boom"}'`, ReasonInvalidOutput},
		{"relative path", `echo '{"path":"a/b"}'`, ReasonInvalidPath},
		{"traversal", `echo '{"path":"/a/../b"}'`, ReasonInvalidPath},
		{"root", `echo '{"path":"/"}'`, ReasonInvalidPath},
		{"NUL in path", `echo '{"path":"/a\u0000b"}'`, ReasonInvalidPath},
		{"non-zero exit", `echo '{"path":"/a"}'; exit 3`, ReasonNonZeroExit},
		{"stdout overflow", `head -c 20000 /dev/zero | tr '\0' x; sleep 30`, ReasonOutputTooLarge},
		{"stderr overflow", `head -c 20000 /dev/zero | tr '\0' x >&2; sleep 30`, ReasonOutputTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOpEnv(t)
			c := e.plugin(t, tc.body)
			res, err := e.ops.Create(context.Background(), c, baseReq())
			asOpError(t, err, OperationCreate, tc.reason)
			must.Eq(t, CreateResult{}, res)
			must.Eq(t, 0, e.ops.locks.len())
		})
	}

	t.Run("start failure", func(t *testing.T) {
		e := newOpEnv(t)
		res, err := e.ops.Create(context.Background(), Candidate{ID: "gone", Path: filepath.Join(e.plugins, "gone")}, baseReq())
		asOpError(t, err, OperationCreate, ReasonStartFailed)
		must.Eq(t, CreateResult{}, res)
		must.Eq(t, 0, e.ops.locks.len())
	})
}

func TestOps_NonZeroExitMessage(t *testing.T) {
	ci.Parallel(t)
	long := strings.Repeat("e", 2*MaxErrorMessage)
	cases := []struct {
		name string
		body string
		want string // exact message, or prefix check when long
	}{
		{"plain exit", `exit 4`, "exit 4"},
		{"error object", `echo '{"error":"disk full"}'; exit 1`, "disk full"},
		{"error object ignores other fields", `echo '{"path":"/a","error":"disk full"}'; exit 1`, "disk full"},
		{"control chars removed", `printf '{"error":"a\\u001b[xb\\nc"}'; exit 1`, "a [xb c"},
		{"not an object", `echo 'disk full'; exit 2`, "exit 2"},
		{"error not a string", `echo '{"error":5}'; exit 2`, "exit 2"},
		{"no error field", `echo '{"msg":"x"}'; exit 2`, "exit 2"},
		{"trailing data", `echo '{"error":"x"} y'; exit 2`, "exit 2"},
		{"empty error", `echo '{"error":""}'; exit 2`, "exit 2"},
		{"stderr ignored", `echo detail >&2; exit 2`, "exit 2"},
		{"bounded", `echo '{"error":"` + long + `"}'; exit 1`, strings.Repeat("e", MaxErrorMessage)},
	}
	for _, op := range []string{OperationCreate, OperationDelete} {
		for _, tc := range cases {
			t.Run(op+"/"+tc.name, func(t *testing.T) {
				e := newOpEnv(t)
				c := e.plugin(t, tc.body)
				var err error
				if op == OperationCreate {
					_, err = e.ops.Create(context.Background(), c, baseReq())
				} else {
					err = e.ops.Delete(context.Background(), c, baseReq(), "/srv/v")
				}
				oe := asOpError(t, err, op, ReasonNonZeroExit)
				must.Eq(t, tc.want, oe.Message)
			})
		}
	}
}

func TestOps_Create_Timeout_KillsProcessGroup(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	e.ops.Timeout = 500 * time.Millisecond
	c := e.plugin(t, `sleep 30 & echo $! > child.pid; wait`)

	res, err := e.ops.Create(context.Background(), c, baseReq())
	asOpError(t, err, OperationCreate, ReasonTimeout)
	must.Eq(t, CreateResult{}, res)
	must.Eq(t, 0, e.ops.locks.len())

	raw, rerr := os.ReadFile(filepath.Join(e.plugins, "child.pid"))
	must.NoError(t, rerr)
	pid, rerr := strconv.Atoi(strings.TrimSpace(string(raw)))
	must.NoError(t, rerr)
	must.Positive(t, pid)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("plugin child process %d survived the timeout", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestOps_Delete_Timeout(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	e.ops.Timeout = 300 * time.Millisecond
	c := e.plugin(t, `exec sleep 30`)
	err := e.ops.Delete(context.Background(), c, baseReq(), "/srv/v")
	asOpError(t, err, OperationDelete, ReasonTimeout)
	must.Eq(t, 0, e.ops.locks.len())
}

// TestOps_CallerDeadlineIsCanceled proves a caller deadline is "canceled"
// and only the operation deadline is "timeout".
func TestOps_CallerDeadlineIsCanceled(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	c := e.plugin(t, `exec sleep 30`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := e.ops.Create(ctx, c, baseReq())
	asOpError(t, err, OperationCreate, ReasonCanceled)
	must.Eq(t, CreateResult{}, res)
	must.Eq(t, 0, e.ops.locks.len())
}

func TestOps_Canceled(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	started := filepath.Join(e.scratch, "started")
	c := e.plugin(t, `echo x > '`+started+`'; exec sleep 30`)

	for _, op := range []string{OperationCreate, OperationDelete} {
		t.Run(op, func(t *testing.T) {
			_ = os.Remove(started)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errc := make(chan error, 1)
			go func() {
				if op == OperationCreate {
					_, err := e.ops.Create(ctx, c, baseReq())
					errc <- err
					return
				}
				errc <- e.ops.Delete(ctx, c, baseReq(), "/srv/v")
			}()
			eventually(t, func() bool { _, err := os.Stat(started); return err == nil }, 10*time.Second)
			cancel()
			asOpError(t, <-errc, op, ReasonCanceled)
			must.Eq(t, 0, e.ops.locks.len())
		})
	}

	t.Run("already canceled starts nothing", func(t *testing.T) {
		_ = os.Remove(started)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := e.ops.Create(ctx, c, baseReq())
		asOpError(t, err, OperationCreate, ReasonCanceled)
		_, serr := os.Stat(started)
		must.True(t, os.IsNotExist(serr))
		must.Eq(t, 0, e.ops.locks.len())
	})
}

func TestOps_Delete(t *testing.T) {
	ci.Parallel(t)

	t.Run("large stdout does not kill the plugin", func(t *testing.T) {
		e := newOpEnv(t)
		marker := filepath.Join(e.scratch, "finished")
		c := e.plugin(t, `head -c 1048576 /dev/zero | tr '\0' x; echo done > '`+marker+`'`)
		must.NoError(t, e.ops.Delete(context.Background(), c, baseReq(), "/srv/v"))
		_, err := os.Stat(marker)
		must.NoError(t, err)
	})

	t.Run("stdout is discarded on success", func(t *testing.T) {
		e := newOpEnv(t)
		for _, out := range []string{`{"error":"ignored"}`, `garbage`, `{"path":"/x"}`} {
			c := e.plugin(t, `echo '`+out+`'`)
			must.NoError(t, e.ops.Delete(context.Background(), c, baseReq(), "/srv/v"), must.Sprint(out))
		}
	})

	t.Run("large stdout then failure is a generic exit", func(t *testing.T) {
		e := newOpEnv(t)
		c := e.plugin(t, `echo '{"error":"hidden"}'; head -c 1048576 /dev/zero | tr '\0' x; exit 7`)
		err := e.ops.Delete(context.Background(), c, baseReq(), "/srv/v")
		oe := asOpError(t, err, OperationDelete, ReasonNonZeroExit)
		must.Eq(t, "exit 7", oe.Message)
	})

	t.Run("stderr overflow kills", func(t *testing.T) {
		e := newOpEnv(t)
		c := e.plugin(t, `head -c 20000 /dev/zero | tr '\0' x >&2; sleep 30`)
		err := e.ops.Delete(context.Background(), c, baseReq(), "/srv/v")
		asOpError(t, err, OperationDelete, ReasonOutputTooLarge)
	})

	t.Run("start failure", func(t *testing.T) {
		e := newOpEnv(t)
		err := e.ops.Delete(context.Background(), Candidate{ID: "gone", Path: filepath.Join(e.plugins, "gone")}, baseReq(), "/srv/v")
		asOpError(t, err, OperationDelete, ReasonStartFailed)
	})

	t.Run("repeated delete succeeds", func(t *testing.T) {
		e := newOpEnv(t)
		dir := filepath.Join(e.volumes, "vol-1")
		must.NoError(t, os.Mkdir(dir, 0o755))
		c := e.plugin(t, `rm -rf "$DHV_CREATED_PATH"`)
		for range 3 {
			must.NoError(t, e.ops.Delete(context.Background(), c, baseReq(), dir))
		}
		_, err := os.Stat(dir)
		must.True(t, os.IsNotExist(err))
		must.Eq(t, 0, e.ops.locks.len())
	})
}

// TestOps_Create_Idempotent retries a create with the same inputs: the
// plugin is called again with byte-identical inputs and no second directory
// appears.
func TestOps_Create_Idempotent(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	dumps := filepath.Join(e.scratch, "dumps")
	must.NoError(t, os.Mkdir(dumps, 0o755))
	c := e.plugin(t, `mkdir -p "$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
n=$(ls '`+dumps+`' | wc -l | tr -d ' ')
env | sort > '`+dumps+`'/$n
printf '{"path":"%s","bytes":0}\n' "$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"`)

	req := baseReq()
	req.Parameters = map[string]string{"z": "<1>", "a": "2", "m": "3"}
	first, err := e.ops.Create(context.Background(), c, req)
	must.NoError(t, err)
	second, err := e.ops.Create(context.Background(), c, req)
	must.NoError(t, err)
	must.Eq(t, first, second)
	must.Eq(t, filepath.Join(e.volumes, "vol-1"), first.Path)

	entries, err := os.ReadDir(e.volumes)
	must.NoError(t, err)
	must.Len(t, 1, entries)

	d0, err := os.ReadFile(filepath.Join(dumps, "0"))
	must.NoError(t, err)
	d1, err := os.ReadFile(filepath.Join(dumps, "1"))
	must.NoError(t, err)
	must.Eq(t, string(d0), string(d1))
	must.StrContains(t, string(d0), `DHV_PARAMETERS={"a":"2","m":"3","z":"<1>"}`)
}

func TestOps_ParameterValuesNeverLeak(t *testing.T) {
	ci.Parallel(t)
	req := baseReq()
	req.Parameters = map[string]string{"token": secretValue, "other": "ok"}
	pad := strings.Repeat("p", MaxErrorMessage-8) // the value straddles the cut

	bodies := map[string]string{
		"error field":         `echo '{"error":"bad ` + secretValue + ` here"}'; exit 1`,
		"error field at cut":  `echo '{"error":"` + pad + secretValue + `"}'; exit 1`,
		"stderr":              `echo ` + secretValue + ` >&2; exit 1`,
		"stderr overflow":     `echo ` + secretValue + ` >&2; head -c 20000 /dev/zero | tr '\0' x >&2; sleep 30`,
		"stdout then invalid": `echo ` + secretValue,
		"invalid path":        `echo '{"path":"` + secretValue + `"}'`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			e := newOpEnv(t)
			c := e.plugin(t, body)
			_, err := e.ops.Create(context.Background(), c, req)
			must.Error(t, err)
			must.StrNotContains(t, err.Error(), secretValue)
			must.StrNotContains(t, err.Error(), secretValue[:12])
			// A cut through the value must not leave a recognisable prefix,
			// which proves redaction runs before truncation.
			must.StrNotContains(t, err.Error(), secretValue[:6])
		})
	}

	t.Run("message shows placeholder", func(t *testing.T) {
		e := newOpEnv(t)
		c := e.plugin(t, `echo '{"error":"bad `+secretValue+` here"}'; exit 1`)
		_, err := e.ops.Create(context.Background(), c, req)
		oe := asOpError(t, err, OperationCreate, ReasonNonZeroExit)
		must.Eq(t, "bad "+redacted+" here", oe.Message)
	})

	t.Run("delete", func(t *testing.T) {
		e := newOpEnv(t)
		c := e.plugin(t, `echo '{"error":"`+secretValue+`"}'; echo `+secretValue+` >&2; exit 1`)
		err := e.ops.Delete(context.Background(), c, req, "/srv/v")
		must.Error(t, err)
		must.StrNotContains(t, err.Error(), secretValue)
	})

	t.Run("rejected input", func(t *testing.T) {
		e := newOpEnv(t)
		c := e.plugin(t, `exit 0`)
		bad := baseReq()
		bad.Parameters = map[string]string{"k": secretValue + "\x00"}
		_, err := e.ops.Create(context.Background(), c, bad)
		asOpError(t, err, OperationCreate, ReasonInvalidInput)
		must.StrNotContains(t, err.Error(), secretValue)
	})
}

func TestOps_InvalidInputRunsNothing(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name   string
		mutate func(*OpRequest)
		reason string
	}{
		{"NUL in namespace", func(r *OpRequest) { r.Namespace = "a\x00b" }, ReasonInvalidInput},
		{"NUL in name", func(r *OpRequest) { r.Name = "a\x00" }, ReasonInvalidInput},
		{"NUL in volume id", func(r *OpRequest) { r.VolumeID = "a\x00" }, ReasonInvalidInput},
		{"NUL in node id", func(r *OpRequest) { r.NodeID = "\x00" }, ReasonInvalidInput},
		{"NUL in node pool", func(r *OpRequest) { r.NodePool = "\x00" }, ReasonInvalidInput},
		{"NUL in parameter key", func(r *OpRequest) { r.Parameters = map[string]string{"a\x00": "v"} }, ReasonInvalidInput},
		{"NUL in parameter value", func(r *OpRequest) { r.Parameters = map[string]string{"a": "v\x00"} }, ReasonInvalidInput},
		{"invalid UTF-8 value", func(r *OpRequest) { r.Parameters = map[string]string{"a": "v\xff"} }, ReasonInvalidInput},
		{"invalid UTF-8 key", func(r *OpRequest) { r.Parameters = map[string]string{"\xc3": "v"} }, ReasonInvalidInput},
		{"parameters too large", func(r *OpRequest) {
			r.Parameters = map[string]string{"a": strings.Repeat("x", MaxParametersBytes)}
		}, ReasonInvalidInput},
		{"negative min", func(r *OpRequest) { r.CapacityMinBytes = -1 }, ReasonInvalidInput},
		{"negative max", func(r *OpRequest) { r.CapacityMaxBytes = -1 }, ReasonInvalidInput},
		{"empty volume id", func(r *OpRequest) { r.VolumeID = "" }, ReasonInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOpEnv(t)
			marker := filepath.Join(e.scratch, "ran")
			c := e.plugin(t, `echo x > '`+marker+`'; echo '{"path":"/a"}'`)
			req := baseReq()
			tc.mutate(&req)

			res, err := e.ops.Create(context.Background(), c, req)
			asOpError(t, err, OperationCreate, tc.reason)
			must.Eq(t, CreateResult{}, res)
			asOpError(t, e.ops.Delete(context.Background(), c, req, "/srv/v"), OperationDelete, tc.reason)

			_, serr := os.Stat(marker)
			must.True(t, os.IsNotExist(serr), must.Sprint("plugin ran"))
			must.Eq(t, 0, e.ops.locks.len())
		})
	}

	t.Run("reserved and invalid ids", func(t *testing.T) {
		e := newOpEnv(t)
		marker := filepath.Join(e.scratch, "ran")
		p := writePlugin(t, e.plugins, "mkdir", `echo x > '`+marker+`'`, 0o755)
		_, err := e.ops.Create(context.Background(), Candidate{ID: BuiltinMkdirID, Path: p}, baseReq())
		asOpError(t, err, OperationCreate, ReasonReservedID)
		err = e.ops.Delete(context.Background(), Candidate{ID: BuiltinMkdirID, Path: p}, baseReq(), "/srv/v")
		asOpError(t, err, OperationDelete, ReasonReservedID)
		_, err = e.ops.Create(context.Background(), Candidate{ID: "a b", Path: p}, baseReq())
		asOpError(t, err, OperationCreate, ReasonInvalidInput)
		_, serr := os.Stat(marker)
		must.True(t, os.IsNotExist(serr))
	})

	t.Run("created path for delete", func(t *testing.T) {
		e := newOpEnv(t)
		marker := filepath.Join(e.scratch, "ran")
		c := e.plugin(t, `echo x > '`+marker+`'`)
		for _, p := range []string{"", "rel", "/a/../b", "/a/", "/", "/a\x00b", "/a\nb", "/" + strings.Repeat("a", MaxPathLen)} {
			err := e.ops.Delete(context.Background(), c, baseReq(), p)
			asOpError(t, err, OperationDelete, ReasonInvalidInput)
		}
		_, serr := os.Stat(marker)
		must.True(t, os.IsNotExist(serr))
	})
}

func TestLockTable(t *testing.T) {
	ci.Parallel(t)
	var lt lockTable
	ctx := context.Background()
	must.NoError(t, lt.acquire(ctx, "A"))
	must.Eq(t, 1, lt.refs("A"))

	got := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		if lt.acquire(ctx, "A") == nil {
			close(got)
			lt.release("A")
		}
	}()
	eventually(t, func() bool { return lt.refs("A") == 2 }, 10*time.Second)
	select {
	case <-got:
		t.Fatal("second acquirer entered while the lock was held")
	default:
	}

	// a different ID is not blocked
	must.NoError(t, lt.acquire(ctx, "B"))
	must.Eq(t, 2, lt.len())
	lt.release("B")

	// a cancelled waiter returns the context error and leaves no reference
	cctx, cancel := context.WithCancel(ctx)
	werr := make(chan error, 1)
	go func() { werr <- lt.acquire(cctx, "A") }()
	eventually(t, func() bool { return lt.refs("A") == 3 }, 10*time.Second)
	cancel()
	must.ErrorIs(t, <-werr, context.Canceled)
	must.Eq(t, 2, lt.refs("A"))

	lt.release("A")
	<-got
	<-done
	must.Eq(t, 0, lt.len())
}

// TestOps_Concurrency runs real plugins that block on a FIFO the test
// controls, so no timing assumptions are needed.
func TestOps_Concurrency(t *testing.T) {
	newBlocking := func(t *testing.T) (opEnv, Candidate) {
		e := newOpEnv(t)
		c := e.plugin(t, `f="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
if ! mkdir "$f.busy" 2>/dev/null; then echo overlap >> "$DHV_VOLUMES_DIR/overlap"; fi
echo started >> "$f.started"
read x <> "$f.fifo"
rmdir "$f.busy"
printf '{"path":"/srv/%s"}\n' "$DHV_VOLUME_ID"`)
		return e, c
	}
	lines := func(path string) int {
		b, _ := os.ReadFile(path)
		return strings.Count(string(b), "\n")
	}
	// the test holds each FIFO open read-write so writes never block and
	// buffered data survives until the plugin reads it
	makeFifo := func(t *testing.T, e opEnv, id string) *os.File {
		p := filepath.Join(e.volumes, id+".fifo")
		must.NoError(t, syscall.Mkfifo(p, 0o600))
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		must.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	started := func(e opEnv, id string) func() bool {
		return func() bool { return lines(filepath.Join(e.volumes, id+".started")) >= 1 }
	}
	type outcome struct {
		res CreateResult
		err error
	}
	spawn := func(e opEnv, c Candidate, id string, wg *sync.WaitGroup) chan outcome {
		ch := make(chan outcome, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := baseReq()
			req.VolumeID = id
			res, err := e.ops.Create(context.Background(), c, req)
			ch <- outcome{res, err}
		}()
		return ch
	}

	t.Run("different ids run concurrently", func(t *testing.T) {
		e, c := newBlocking(t)
		fa, fb := makeFifo(t, e, "A"), makeFifo(t, e, "B")
		var wg sync.WaitGroup
		defer wg.Wait()
		ra, rb := spawn(e, c, "A", &wg), spawn(e, c, "B", &wg)
		// both are blocked in their plugin at the same time
		eventually(t, started(e, "A"), 20*time.Second)
		eventually(t, started(e, "B"), 20*time.Second)
		_, _ = fa.WriteString("go\n")
		_, _ = fb.WriteString("go\n")
		must.NoError(t, (<-ra).err)
		must.NoError(t, (<-rb).err)
		must.Eq(t, 0, e.ops.locks.len())
	})

	t.Run("same id is serialised", func(t *testing.T) {
		e, c := newBlocking(t)
		f := makeFifo(t, e, "A")
		var wg sync.WaitGroup
		defer wg.Wait()
		r1 := spawn(e, c, "A", &wg)
		eventually(t, started(e, "A"), 20*time.Second)
		r2 := spawn(e, c, "A", &wg)
		// the second call is waiting for the lock, so its plugin has not run
		eventually(t, func() bool { return e.ops.locks.refs("A") == 2 }, 20*time.Second)
		must.Eq(t, 1, lines(filepath.Join(e.volumes, "A.started")))
		_, _ = f.WriteString("go\n")
		must.NoError(t, (<-r1).err)
		eventually(t, func() bool { return lines(filepath.Join(e.volumes, "A.started")) == 2 }, 20*time.Second)
		_, _ = f.WriteString("go\n")
		must.NoError(t, (<-r2).err)
		_, err := os.Stat(filepath.Join(e.volumes, "overlap"))
		must.True(t, os.IsNotExist(err), must.Sprint("plugins overlapped"))
		must.Eq(t, 0, e.ops.locks.len())
	})

	t.Run("waiter cancelled while queued starts nothing and lock survives", func(t *testing.T) {
		e, c := newBlocking(t)
		f := makeFifo(t, e, "A")
		var wg sync.WaitGroup
		defer wg.Wait()
		r1 := spawn(e, c, "A", &wg)
		eventually(t, started(e, "A"), 20*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := baseReq()
		req.VolumeID = "A"
		werr := make(chan error, 1)
		go func() { _, err := e.ops.Create(ctx, c, req); werr <- err }()
		eventually(t, func() bool { return e.ops.locks.refs("A") == 2 }, 20*time.Second)
		cancel()
		asOpError(t, <-werr, OperationCreate, ReasonCanceled)
		must.Eq(t, 1, e.ops.locks.refs("A"))
		must.Eq(t, 1, lines(filepath.Join(e.volumes, "A.started")))

		_, _ = f.WriteString("go\n")
		must.NoError(t, (<-r1).err)
		must.Eq(t, 0, e.ops.locks.len())
	})
}

// TestOps_FailureReleasesLock checks that the next call on the same ID is not
// blocked after each kind of failure.
func TestOps_FailureReleasesLock(t *testing.T) {
	ci.Parallel(t)
	e := newOpEnv(t)
	e.ops.Timeout = 300 * time.Millisecond
	for _, body := range []string{`exit 1`, `echo bad`, `exec sleep 30`} {
		c := e.plugin(t, body)
		_, err := e.ops.Create(context.Background(), c, baseReq())
		must.Error(t, err)
		must.Eq(t, 0, e.ops.locks.len())
	}
	c := e.plugin(t, `echo '{"path":"/a"}'`)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := e.ops.Create(ctx, c, baseReq())
	must.NoError(t, err)
	must.Eq(t, "/a", res.Path)
}

func TestOpError_Error(t *testing.T) {
	ci.Parallel(t)
	must.Eq(t, "host volume plugin create: timeout", (&OpError{Op: "create", Reason: ReasonTimeout}).Error())
	must.Eq(t, "host volume plugin delete: nonzero_exit: exit 2",
		(&OpError{Op: "delete", Reason: ReasonNonZeroExit, Message: "exit 2"}).Error())
}

// TestRunPlugin_TruncateStdout covers the additive option directly.
func TestRunPlugin_TruncateStdout(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	p := writePlugin(t, dir, "plug", `head -c 5000 /dev/zero | tr '\0' x; echo ok > '`+marker+`'`, 0o755)

	res, err := runPlugin(context.Background(), p, nil, nil, Limits{Stdout: 100, Stderr: 100, TruncateStdout: true})
	must.NoError(t, err)
	must.False(t, res.Overflow)
	must.True(t, res.StdoutTruncated)
	must.Len(t, 100, res.Stdout)
	_, serr := os.Stat(marker)
	must.NoError(t, serr)

	// the default still kills
	_ = os.Remove(marker)
	res, _ = runPlugin(context.Background(), p, nil, nil, Limits{Stdout: 100, Stderr: 100})
	must.True(t, res.Overflow)
	must.False(t, res.StdoutTruncated)
}
