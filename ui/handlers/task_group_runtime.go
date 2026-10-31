package handlers

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/taskgrouppartials"
)

func (h *JobsHandler) enrichTaskGroupRuntime(r *http.Request, client *api.Client, queryOpts *api.QueryOptions, jobName, groupName string, detail *taskgrouppartials.TaskGroupDetail) {
	queryOpts.Region = GetRegionFromRequest(r)
	detail.AllocationStatus = taskGroupQueryOption(r, "status", []string{"all", "pending", "running", "complete", "failed", "lost"}, "all")
	detail.AllocationSort = taskGroupQueryOption(r, "sort", []string{"created", "status", "node"}, "created")
	detail.AllocationOrder = taskGroupQueryOption(r, "order", []string{"asc", "desc"}, "desc")

	allocations, _, err := client.Jobs().Allocations(jobName, true, queryOpts)
	if err != nil {
		detail.AllocationsError = "the cluster request failed"
	} else {
		detail.AllocationsReady = true
		detail.Allocations = projectTaskGroupAllocations(allocations, groupName, detail.AllocationStatus, detail.AllocationSort, detail.AllocationOrder, detail.Namespace, detail.Region)
	}

	status, _, err := client.Jobs().ScaleStatus(jobName, queryOpts)
	if err != nil {
		detail.ScaleStatusError = "the cluster request failed"
		return
	}
	detail.ScaleStatusReady = true
	if status == nil {
		return
	}
	groupStatus, ok := status.TaskGroups[groupName]
	if !ok {
		return
	}
	detail.ScaleStatus = projectTaskGroupScaleStatus(groupStatus)
}

func taskGroupQueryOption(r *http.Request, key string, allowed []string, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func projectTaskGroupAllocations(allocations []*api.AllocationListStub, groupName, status, sortBy, order, namespace, region string) []taskgrouppartials.RuntimeAllocation {
	result := make([]taskgrouppartials.RuntimeAllocation, 0)
	for _, allocation := range allocations {
		if allocation == nil || allocation.TaskGroup != groupName || (status != "all" && allocation.ClientStatus != status) {
			continue
		}
		result = append(result, taskgrouppartials.RuntimeAllocation{
			ID:         allocation.ID,
			ShortID:    shortIdentifier(allocation.ID),
			Name:       allocation.Name,
			NodeName:   allocation.NodeName,
			Status:     allocation.ClientStatus,
			JobVersion: allocation.JobVersion,
			Created:    unixNanoTime(allocation.CreateTime),
			DetailURL:  scopedUIURL("/ui/allocations/"+allocation.ID, namespace, region),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		comparison := 0
		switch sortBy {
		case "status":
			comparison = strings.Compare(result[i].Status, result[j].Status)
		case "node":
			comparison = strings.Compare(result[i].NodeName, result[j].NodeName)
		default:
			comparison = result[i].Created.Compare(result[j].Created)
		}
		if comparison == 0 {
			comparison = strings.Compare(result[i].ID, result[j].ID)
		}
		if order == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
	return result
}

func projectTaskGroupScaleStatus(status api.TaskGroupScaleStatus) *taskgrouppartials.RuntimeScaleStatus {
	result := &taskgrouppartials.RuntimeScaleStatus{
		Desired: status.Desired, Placed: status.Placed, Running: status.Running,
		Healthy: status.Healthy, Unhealthy: status.Unhealthy,
		Events: make([]taskgrouppartials.RuntimeScalingEvent, 0, len(status.Events)),
	}
	for _, event := range status.Events {
		target := "—"
		if event.Count != nil {
			target = formatInt64(*event.Count)
		}
		evaluation := ""
		if event.EvalID != nil {
			evaluation = *event.EvalID
		}
		eventTime := time.Time{}
		if event.Time <= uint64(math.MaxInt64) {
			eventTime = unixNanoTime(int64(event.Time))
		}
		result.Events = append(result.Events, taskgrouppartials.RuntimeScalingEvent{
			Target: target, PreviousCount: event.PreviousCount, Message: event.Message,
			Error: event.Error, Evaluation: evaluation, Time: eventTime, CreateIndex: event.CreateIndex,
		})
	}
	sort.Slice(result.Events, func(i, j int) bool {
		if result.Events[i].CreateIndex == result.Events[j].CreateIndex {
			return result.Events[i].Evaluation > result.Events[j].Evaluation
		}
		return result.Events[i].CreateIndex > result.Events[j].CreateIndex
	})
	return result
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
