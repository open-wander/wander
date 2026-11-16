// -------------------------------------------------------------------------------
// Integration Test Helpers
//
// Project: Wander
//
// Polling for tests that read state the cluster converges on. Only reads go
// through it: a write is sent once, so a slow response is never resent to
// another server.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// pollGap is the pause between two attempts of an eventually check.
const pollGap = time.Second

// eventually retries check until it returns nil, and fails the test with the
// last error check returned if timeout passes first.
func eventually(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(check),
		wait.Timeout(timeout),
		wait.Gap(pollGap),
	))
}
