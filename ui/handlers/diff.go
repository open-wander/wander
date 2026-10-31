package handlers

import (
	"fmt"
	"strings"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

const (
	changeAdded   = "Added"
	changeEdited  = "Edited"
	changeDeleted = "Deleted"
	changeNone    = "None"
)

func projectVersionDiff(jobID string, newer, older uint64, diff *api.JobDiff) jobspartials.VersionDiff {
	result := jobspartials.VersionDiff{
		JobID:      jobID,
		OldVersion: older,
		NewVersion: newer,
	}
	if diff == nil {
		return result
	}

	if diff.Type == changeAdded || diff.Type == changeDeleted {
		addVersionChange(&result, jobspartials.VersionChange{Type: diff.Type, Scope: "Job", Name: diff.ID})
		return result
	}

	appendFieldChanges(&result, "Job", diff.Fields)
	appendObjectChanges(&result, "Job", diff.Objects)
	for _, group := range diff.TaskGroups {
		appendTaskGroupChanges(&result, group)
	}
	return result
}

func addVersionChange(diff *jobspartials.VersionDiff, change jobspartials.VersionChange) {
	change.Type = normalizedChangeType(change.Type)
	if change.Type == "" {
		return
	}
	diff.Changes = append(diff.Changes, change)
	switch change.Type {
	case changeAdded:
		diff.Counts.Added++
	case changeDeleted:
		diff.Counts.Deleted++
	default:
		diff.Counts.Edited++
	}
}

func normalizedChangeType(changeType string) string {
	switch changeType {
	case "", changeNone:
		return ""
	case changeAdded, changeEdited, changeDeleted:
		return changeType
	default:
		return changeEdited
	}
}

func appendFieldChanges(diff *jobspartials.VersionDiff, scope string, fields []*api.FieldDiff) {
	for _, field := range fields {
		if field == nil || normalizedChangeType(field.Type) == "" {
			continue
		}
		addVersionChange(diff, jobspartials.VersionChange{
			Type:        field.Type,
			Scope:       scope,
			Name:        field.Name,
			Old:         field.Old,
			New:         field.New,
			Annotations: field.Annotations,
		})
	}
}

func appendObjectChanges(diff *jobspartials.VersionDiff, scope string, objects []*api.ObjectDiff) {
	for _, object := range objects {
		if object == nil {
			continue
		}
		typeName := normalizedChangeType(object.Type)
		if typeName == changeAdded || typeName == changeDeleted {
			addVersionChange(diff, jobspartials.VersionChange{Type: typeName, Scope: scope, Name: object.Name})
			continue
		}

		before := len(diff.Changes)
		objectScope := joinChangeScope(scope, object.Name)
		appendFieldChanges(diff, objectScope, object.Fields)
		appendObjectChanges(diff, objectScope, object.Objects)
		if typeName == changeEdited && len(diff.Changes) == before {
			addVersionChange(diff, jobspartials.VersionChange{Type: typeName, Scope: scope, Name: object.Name})
		}
	}
}

func appendTaskGroupChanges(diff *jobspartials.VersionDiff, group *api.TaskGroupDiff) {
	if group == nil {
		return
	}
	groupName := fmt.Sprintf("Task group %q", group.Name)
	typeName := normalizedChangeType(group.Type)
	if typeName == changeAdded || typeName == changeDeleted {
		addVersionChange(diff, jobspartials.VersionChange{Type: typeName, Scope: "Job", Name: groupName})
		return
	}

	before := len(diff.Changes)
	appendFieldChanges(diff, groupName, group.Fields)
	appendObjectChanges(diff, groupName, group.Objects)
	for _, task := range group.Tasks {
		appendTaskChanges(diff, groupName, task)
	}
	if typeName == changeEdited && len(diff.Changes) == before {
		addVersionChange(diff, jobspartials.VersionChange{Type: typeName, Scope: "Job", Name: groupName})
	}
}

func appendTaskChanges(diff *jobspartials.VersionDiff, groupScope string, task *api.TaskDiff) {
	if task == nil {
		return
	}
	taskName := fmt.Sprintf("Task %q", task.Name)
	typeName := normalizedChangeType(task.Type)
	if typeName == changeAdded || typeName == changeDeleted {
		addVersionChange(diff, jobspartials.VersionChange{
			Type: typeName, Scope: groupScope, Name: taskName, Annotations: task.Annotations,
		})
		return
	}

	before := len(diff.Changes)
	taskScope := joinChangeScope(groupScope, taskName)
	appendFieldChanges(diff, taskScope, task.Fields)
	appendObjectChanges(diff, taskScope, task.Objects)
	if typeName == changeEdited && len(diff.Changes) == before {
		addVersionChange(diff, jobspartials.VersionChange{
			Type: typeName, Scope: groupScope, Name: taskName, Annotations: task.Annotations,
		})
	}
}

func joinChangeScope(parts ...string) string {
	nonEmpty := parts[:0]
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, " / ")
}
