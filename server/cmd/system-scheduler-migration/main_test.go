package main

import (
	"fmt"
	"testing"

	"github.com/hibiken/asynq"
)

type inspectorFixture struct {
	queues    map[string]map[string][]*asynq.TaskInfo
	pageSize  int
	maxListed int
}

func (f *inspectorFixture) Queues() ([]string, error) {
	result := make([]string, 0, len(f.queues))
	for queue := range f.queues {
		result = append(result, queue)
	}
	return result, nil
}
func (f *inspectorFixture) ListPendingTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return f.list(queue, "pending", page, size)
}
func (f *inspectorFixture) ListScheduledTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return f.list(queue, "scheduled", page, size)
}
func (f *inspectorFixture) ListRetryTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return f.list(queue, "retry", page, size)
}
func (f *inspectorFixture) list(queue, state string, page, size int) ([]*asynq.TaskInfo, error) {
	items := f.queues[queue][state]
	start := (page - 1) * size
	if start >= len(items) {
		return nil, nil
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	if end > f.maxListed {
		f.maxListed = end
	}
	return append([]*asynq.TaskInfo(nil), items[start:end]...), nil
}
func (f *inspectorFixture) DeleteTask(queue, id string) error {
	for state, items := range f.queues[queue] {
		for i, item := range items {
			if item.ID == id {
				f.queues[queue][state] = append(items[:i], items[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("missing task %s", id)
}

func TestCleanupOldTasksScansBeyondFirstPageAndDoesNotSkipAfterDeletion(t *testing.T) {
	fixture := &inspectorFixture{queues: map[string]map[string][]*asynq.TaskInfo{"default": {"pending": {}, "scheduled": {}, "retry": {}}}}
	for i := 0; i < 225; i++ {
		taskType := "unrelated"
		if i == 120 || i == 121 || i == 220 {
			taskType = oldTaskType
		}
		fixture.queues["default"]["pending"] = append(fixture.queues["default"]["pending"], &asynq.TaskInfo{ID: fmt.Sprintf("task-%03d", i), Type: taskType})
	}
	deleted, remaining, err := cleanupOldTasks(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 || remaining != 0 {
		t.Fatalf("deleted=%d remaining=%d", deleted, remaining)
	}
	if fixture.maxListed <= 100 {
		t.Fatalf("cleanup never scanned beyond first page: %d", fixture.maxListed)
	}
	if len(fixture.queues["default"]["pending"]) != 222 {
		t.Fatalf("unrelated tasks changed: %d", len(fixture.queues["default"]["pending"]))
	}
}
