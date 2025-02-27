// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"github.com/open-wander/wander/helper/stats"
)

// PIDs holds all of a task's pids and their cpu percentage calculators
type PIDs map[int]*PID

// PID holds one task's pid and it's cpu percentage calculator
type PID struct {
	PID           int
	StatsTotalCPU *stats.CpuStats
	StatsUserCPU  *stats.CpuStats
	StatsSysCPU   *stats.CpuStats
}

func NewPID(pid int) *PID {
	return &PID{
		PID:           pid,
		StatsTotalCPU: stats.NewCpuStats(),
		StatsUserCPU:  stats.NewCpuStats(),
		StatsSysCPU:   stats.NewCpuStats(),
	}
}
