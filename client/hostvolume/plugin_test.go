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
	"syscall"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

func writePlugin(t *testing.T, dir, name, body string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	must.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), mode))
	must.NoError(t, os.Chmod(p, mode))
	return p
}

func TestDiscover_Filtering(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	writePlugin(t, dir, "good", "echo 1", 0o755)
	writePlugin(t, dir, "noexec", "echo 1", 0o644)
	writePlugin(t, dir, ".hidden", "echo 1", 0o755)
	writePlugin(t, dir, "bad name", "echo 1", 0o755)
	writePlugin(t, dir, "mkdir", "echo 1", 0o755)
	must.NoError(t, os.Mkdir(filepath.Join(dir, "adir"), 0o755))
	must.NoError(t, os.Symlink(filepath.Join(dir, "good"), filepath.Join(dir, "link")))
	must.NoError(t, syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o755))
	// a nested plugin must not be found
	must.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	writePlugin(t, filepath.Join(dir, "sub"), "nested", "echo 1", 0o755)

	cands, diags := Discover(dir)
	must.Len(t, 1, cands)
	must.Eq(t, "good", cands[0].ID)
	must.Eq(t, filepath.Join(dir, "good"), cands[0].Path)

	got := map[string]string{}
	for _, d := range diags {
		got[d.ID] = d.Reason
	}
	must.Eq(t, map[string]string{
		"noexec":   ReasonNotExecutable,
		"bad name": ReasonInvalidID,
		"mkdir":    ReasonReservedID,
	}, got)
}

func TestDiscover_EmptyMissing(t *testing.T) {
	ci.Parallel(t)
	for name, dir := range map[string]string{
		"unset":   "",
		"missing": filepath.Join(t.TempDir(), "nope"),
		"empty":   t.TempDir(),
	} {
		cands, diags := Discover(dir)
		must.Len(t, 0, cands, must.Sprint(name))
		must.Len(t, 0, diags, must.Sprint(name))
	}
}

func TestDiscover_DeterministicOrder(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	for _, n := range []string{"c", "a", "b", "z9", "Zed"} {
		writePlugin(t, dir, n, "echo 1", 0o755)
	}
	want := []string{"Zed", "a", "b", "c", "z9"}
	real, err := os.ReadDir(dir)
	must.NoError(t, err)
	for _, reverse := range []bool{false, true} {
		list := func(string) ([]os.DirEntry, error) {
			out := append([]os.DirEntry(nil), real...)
			if reverse {
				for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
					out[i], out[j] = out[j], out[i]
				}
			}
			return out, nil
		}
		cands, _ := DiscoverWith(dir, list)
		var ids []string
		for _, c := range cands {
			ids = append(ids, c.ID)
		}
		must.Eq(t, want, ids)
	}
}

func TestDiscover_CandidateCap(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	for i := range MaxCandidates + 3 {
		writePlugin(t, dir, "p"+string(rune('a'+i/26))+string(rune('a'+i%26)), "echo 1", 0o755)
	}
	cands, diags := Discover(dir)
	must.Len(t, MaxCandidates, cands)
	must.Len(t, 3, diags)
	must.Eq(t, ReasonTooManyPlugins, diags[0].Reason)
}

func TestRunner_Fingerprint(t *testing.T) {
	ci.Parallel(t)
	r := Runner{Timeout: 5 * time.Second, Limits: Limits{Stdout: 128, Stderr: 64}}
	short := Runner{Timeout: 200 * time.Millisecond, Limits: r.Limits}

	cases := []struct {
		name    string
		body    string
		mode    os.FileMode
		healthy bool
		version string
		reason  string
	}{
		{"valid", `echo 1.2.3`, 0o755, true, "1.2.3", ""},
		{"valid no newline", `printf v1+meta_2`, 0o755, true, "v1+meta_2", ""},
		{"non-executable", `echo 1.0`, 0o644, false, "", ReasonStartFailed},
		{"empty output", `exit 0`, 0o755, false, "", ReasonInvalidVersion},
		{"multi-line", `printf '1.0\n2.0\n'`, 0o755, false, "", ReasonInvalidVersion},
		{"bad characters", `echo 'a b;rm'`, 0o755, false, "", ReasonInvalidVersion},
		{"control characters", `printf '1.0\033[31m\n'`, 0o755, false, "", ReasonInvalidVersion},
		{"too long version", `echo ` + strings.Repeat("1", MaxVersionLen+1), 0o755, false, "", ReasonInvalidVersion},
		{"non-zero exit", `echo 1.0; echo boom >&2; exit 3`, 0o755, false, "", ReasonNonZeroExit},
		{"timeout", `exec sleep 30`, 0o755, false, "", ReasonTimeout},
		{"timeout with grandchild", `sleep 30 & wait`, 0o755, false, "", ReasonTimeout},
		{"oversized stdout", `exec yes 1234567890`, 0o755, false, "", ReasonOutputTooLarge},
		{"oversized stderr", `exec yes 1 >&2`, 0o755, false, "", ReasonOutputTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writePlugin(t, dir, "plug", tc.body, tc.mode)
			start := time.Now()
			runner := r
			if tc.reason == ReasonTimeout {
				runner = short
			}
			res := runner.Fingerprint(context.Background(), Candidate{ID: "plug", Path: p})
			must.Less(t, 4*time.Second, time.Since(start))
			must.Eq(t, "plug", res.ID)
			must.Eq(t, tc.healthy, res.Healthy)
			if tc.healthy {
				must.Eq(t, tc.version, res.Version)
				must.Nil(t, res.Diag)
				return
			}
			must.NotNil(t, res.Diag)
			must.Eq(t, tc.reason, res.Diag.Reason)
			must.LessEq(t, MaxDiagDetail+32, len(res.Diag.Detail))
			for _, c := range res.Diag.Detail {
				must.True(t, c >= ' ' && c != 0x7f)
			}
		})
	}
}

func TestRunner_Fingerprint_DiagnosticBounded(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	p := writePlugin(t, dir, "plug", "printf '\\033[1m\\001'; head -c 900 /dev/zero | tr '\\0' x >&2; exit 1", 0o755)
	res := Runner{Limits: Limits{Stderr: 900}}.Fingerprint(context.Background(), Candidate{ID: "plug", Path: p})
	must.Eq(t, ReasonNonZeroExit, res.Diag.Reason)
	must.LessEq(t, MaxDiagDetail+32, len(res.Diag.Detail))
	must.StrContains(t, res.Diag.Detail, "exit 1")
}

func TestRunner_Fingerprint_ArgvAndEnv(t *testing.T) {
	t.Setenv("DHV_TEST_SENTINEL", "leak")
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	p := writePlugin(t, dir, "plug",
		`{ echo "argc=$#"; echo "arg1=$1"; echo "pwd=$(pwd -P)"; env | sort; } > '`+out+`'; echo 1.0`, 0o755)
	res := Runner{}.Fingerprint(context.Background(), Candidate{ID: "plug", Path: p})
	must.True(t, res.Healthy)

	b, err := os.ReadFile(out)
	must.NoError(t, err)
	text := string(b)
	must.StrContains(t, text, "argc=1\n")
	must.StrContains(t, text, "arg1=fingerprint\n")
	must.StrNotContains(t, text, "leak")
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		k, _, ok := strings.Cut(line, "=")
		must.True(t, ok)
		switch k {
		case "argc", "arg1", "pwd":
		case "DHV_OPERATION", "PATH":
		case "PWD", "SHLVL", "_", "OLDPWD": // set by the shell itself
		default:
			t.Fatalf("unexpected variable in plugin environment: %q", k)
		}
	}
	must.StrContains(t, text, "DHV_OPERATION=fingerprint\n")
}

func TestRunPlugin_NilEnvIsNotInherited(t *testing.T) {
	t.Setenv("DHV_TEST_SENTINEL", "leak")
	dir := t.TempDir()
	p := writePlugin(t, dir, "plug", `echo "${DHV_TEST_SENTINEL:-none}"`, 0o755)
	res, err := runPlugin(context.Background(), p, nil, nil, Limits{Stdout: 64, Stderr: 64})
	must.NoError(t, err)
	must.Eq(t, "none\n", string(res.Stdout))
}

func TestRunner_Fingerprint_Canceled(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	p := writePlugin(t, dir, "plug", `exec sleep 30`, 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res := Runner{Timeout: 20 * time.Second}.Fingerprint(ctx, Candidate{ID: "plug", Path: p})
	must.Eq(t, ReasonCanceled, res.Diag.Reason)
}

func TestVersionAndID(t *testing.T) {
	ci.Parallel(t)
	for _, v := range []string{"1", "v1.2.3-rc.1+b5", "a_b"} {
		_, err := ParseVersion([]byte(v + "\n"))
		must.NoError(t, err, must.Sprint(v))
	}
	for _, v := range []string{"", "\n", "\n\n", "a\nb", "a b", "a/b", strings.Repeat("x", 65), "é"} {
		_, err := ParseVersion([]byte(v))
		must.Error(t, err, must.Sprint(v))
	}
	for _, id := range []string{"a", "my-plugin.sh", "A_1"} {
		must.NoError(t, ValidateID(id))
	}
	for _, id := range []string{"", ".a", "a b", "a/b", strings.Repeat("x", 65)} {
		must.Error(t, ValidateID(id), must.Sprint(id))
	}
}

// TestRunner_Fingerprint_TimeoutKillsProcessGroup proves a timeout kills the
// plugin's children too, not only the plugin process itself.
func TestRunner_Fingerprint_TimeoutKillsProcessGroup(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	p := writePlugin(t, dir, "plug", `sleep 30 & echo $! > child.pid; wait`, 0o755)

	runner := Runner{Timeout: 500 * time.Millisecond, Limits: Limits{Stdout: 128, Stderr: 64}}
	res := runner.Fingerprint(context.Background(), Candidate{ID: "plug", Path: p})
	must.False(t, res.Healthy)
	must.NotNil(t, res.Diag)
	must.Eq(t, ReasonTimeout, res.Diag.Reason)

	raw, err := os.ReadFile(pidFile)
	must.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	must.NoError(t, err)
	must.Positive(t, pid)

	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("plugin child process %d survived the timeout", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
