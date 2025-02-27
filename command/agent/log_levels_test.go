// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"testing"

	"github.com/hashicorp/logutils"
	"github.com/open-wander/wander/ci"
)

func TestLevelFilter(t *testing.T) {
	ci.Parallel(t)

	filt := LevelFilter()
	filt.Levels = []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"}
	level := logutils.LogLevel("INFO")

	// LevelFilter regards INFO as valid level
	if !ValidateLevelFilter(level, filt) {
		t.Fatalf("expected valid LogLevel, %s was invalid", level)
	}

	level = logutils.LogLevel("FOO")

	// LevelFilter regards FOO as invalid level
	if ValidateLevelFilter(level, filt) {
		t.Fatalf("expected invalid LogLevel, %s was valid", level)
	}
}
