package allocationspartials

import (
	"context"
	"strings"
	"testing"
)

func TestAllocationRestartControlsExposeInheritedTargets(t *testing.T) {
	allocation := AllocationDetail{
		ID: "alloc-1",
		Tasks: []TaskStatus{
			{Name: "web task#1", State: "running"},
			{Name: "prepare", State: "dead"},
		},
	}

	var detail strings.Builder
	if err := DetailContent(allocation).Render(context.Background(), &detail); err != nil {
		t.Fatalf("render allocation detail: %v", err)
	}
	if !strings.Contains(detail.String(), "data-all-tasks-restart") ||
		!strings.Contains(detail.String(), "Restart Running Tasks") {
		t.Fatalf("allocation detail is missing restart-running or restart-all controls: %s", detail.String())
	}

	var tasks strings.Builder
	if err := TasksTab(allocation).Render(context.Background(), &tasks); err != nil {
		t.Fatalf("render allocation tasks: %v", err)
	}
	if !strings.Contains(tasks.String(), `data-task-restart="web task#1"`) ||
		!strings.Contains(tasks.String(), `task=web+task%231`) {
		t.Fatalf("running task restart control is missing or not safely encoded: %s", tasks.String())
	}
	if strings.Contains(tasks.String(), `data-task-restart="prepare"`) {
		t.Fatalf("completed task must not expose the running-task restart action: %s", tasks.String())
	}
}
