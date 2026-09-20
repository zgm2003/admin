package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"admin/server/internal/config"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

const (
	modeForward = "forward"
	modeCleanup = "cleanup"
	oldTaskType = "message:notificationtask:dispatch:v1"
)

type taskInspector interface {
	Queues() ([]string, error)
	ListPendingTasks(string, int, int) ([]*asynq.TaskInfo, error)
	ListScheduledTasks(string, int, int) ([]*asynq.TaskInfo, error)
	ListRetryTasks(string, int, int) ([]*asynq.TaskInfo, error)
	DeleteTask(string, string) error
}

type asynqInspector struct{ value *asynq.Inspector }

func (i asynqInspector) Queues() ([]string, error) { return i.value.Queues() }
func (i asynqInspector) ListPendingTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return i.value.ListPendingTasks(queue, asynq.Page(page), asynq.PageSize(size))
}
func (i asynqInspector) ListScheduledTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return i.value.ListScheduledTasks(queue, asynq.Page(page), asynq.PageSize(size))
}
func (i asynqInspector) ListRetryTasks(queue string, page, size int) ([]*asynq.TaskInfo, error) {
	return i.value.ListRetryTasks(queue, asynq.Page(page), asynq.PageSize(size))
}
func (i asynqInspector) DeleteTask(queue, id string) error { return i.value.DeleteTask(queue, id) }

func main() {
	if err := runMain(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMain() error {
	mode, err := parseMode(os.Args[1:])
	if err != nil {
		return err
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, settings.RedisURL, mode)
}

func parseMode(args []string) (string, error) {
	flags := flag.NewFlagSet("system-scheduler-migration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", modeForward, "migration mode: forward|cleanup")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("system scheduler migration accepts only -mode")
	}
	if *mode != modeForward && *mode != modeCleanup {
		return "", fmt.Errorf("mode must be %s or %s", modeForward, modeCleanup)
	}
	return *mode, nil
}

func run(ctx context.Context, redisURL, mode string) error {
	redisOptions, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return err
	}
	inspector := asynq.NewInspector(redisOptions)
	defer func() { _ = inspector.Close() }()
	deleted, remaining, err := cleanupOldTasks(asynqInspector{value: inspector})
	if err != nil {
		return err
	}
	if remaining != 0 {
		return fmt.Errorf("old notification task type remains in Redis: %d", remaining)
	}
	_, _ = fmt.Fprintf(os.Stdout, "cleanup mode=%s taskType=%s deleted=%d remaining=%d\n", mode, oldTaskType, deleted, remaining)
	return nil
}

func cleanupOldTasks(inspector taskInspector) (deleted, remaining int, err error) {
	queues, err := inspector.Queues()
	if err != nil {
		return 0, 0, fmt.Errorf("list Asynq queues: %w", err)
	}
	sort.Strings(queues)
	for _, queue := range queues {
		for _, list := range []func(string, int, int) ([]*asynq.TaskInfo, error){inspector.ListPendingTasks, inspector.ListScheduledTasks, inspector.ListRetryTasks} {
			for {
				found := false
				for page := 1; ; page++ {
					items, listErr := list(queue, page, 100)
					if listErr != nil {
						return deleted, remaining, fmt.Errorf("list old Asynq tasks in queue %s: %w", queue, listErr)
					}
					for _, item := range items {
						if item.Type != oldTaskType {
							continue
						}
						found = true
						if deleteErr := inspector.DeleteTask(queue, item.ID); deleteErr != nil {
							return deleted, remaining, fmt.Errorf("delete old Asynq task %s in queue %s: %w", item.ID, queue, deleteErr)
						}
						deleted++
					}
					// Once deletion changes page boundaries, restart the scan at page 1.
					if found || len(items) < 100 {
						break
					}
				}
				if !found {
					break
				}
			}
		}
	}
	for _, queue := range queues {
		for _, list := range []func(string, int, int) ([]*asynq.TaskInfo, error){inspector.ListPendingTasks, inspector.ListScheduledTasks, inspector.ListRetryTasks} {
			for page := 1; ; page++ {
				items, listErr := list(queue, page, 100)
				if listErr != nil {
					return deleted, remaining, listErr
				}
				for _, item := range items {
					if item.Type == oldTaskType {
						remaining++
					}
				}
				if len(items) < 100 {
					break
				}
			}
		}
	}
	return deleted, remaining, nil
}
