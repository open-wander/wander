// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stats

import (
	"math"
	"os"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/stretchr/testify/assert"
)

func TestHostStats_CPU(t *testing.T) {
	ci.Parallel(t)

	assert := assert.New(t)
	assert.Nil(Init(0))

	logger := testlog.HCLogger(t)
	cwd, err := os.Getwd()
	assert.Nil(err)
	hs := NewHostStatsCollector(logger, cwd, nil)

	// Collect twice so we can calculate percents we need to generate some work
	// so that the cpu values change
	assert.Nil(hs.Collect())
	total := 0
	for i := 1; i < 1000000000; i++ {
		total *= i
		total = total % i
	}
	assert.Nil(hs.Collect())
	stats := hs.Stats()

	assert.NotZero(stats.CPUTicksConsumed)
	assert.NotZero(len(stats.CPU))

	for _, cpu := range stats.CPU {
		assert.False(math.IsNaN(cpu.Idle))
		assert.False(math.IsNaN(cpu.TotalPercent))
		assert.False(math.IsNaN(cpu.TotalTicks))
		assert.False(math.IsNaN(cpu.System))
		assert.False(math.IsNaN(cpu.User))

		assert.False(math.IsInf(cpu.Idle, 0))
		assert.False(math.IsInf(cpu.TotalPercent, 0))
		assert.False(math.IsInf(cpu.TotalTicks, 0))
		assert.False(math.IsInf(cpu.System, 0))
		assert.False(math.IsInf(cpu.User, 0))
	}
}
