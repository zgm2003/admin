package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/system/operationLog"
	"admin/server/internal/module/system/scheduler"

	"github.com/hibiken/asynq"
)

func TestOpenWorkerRedisKeepsStartupContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := openWorkerRedis(ctx, "redis://127.0.0.1:1/0")
	if err == nil {
		t.Fatal("expected Redis startup to fail")
	}
	for _, want := range []string{"open Worker Redis", "ping Redis"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q context", err, want)
		}
	}
}

func TestBuildWorkerSettingServiceRejectsIncompleteCacheGenerationDependencies(t *testing.T) {
	if _, err := buildWorkerSettingService(nil, nil, nil, nil, discardLogger()); err == nil {
		t.Fatal("worker setting service accepted incomplete cache generation dependencies")
	}
}

type workerOperationLogProcessor struct {
	processed string
}

func (p *workerOperationLogProcessor) Process(_ context.Context, payload operationlog.TaskPayload) error {
	p.processed = payload.RequestID
	return nil
}

func TestBuildWorkerMuxRegistersOperationLogAndSchedulerTasks(t *testing.T) {
	operationProcessor := &workerOperationLogProcessor{}
	notificationProcessor := &fakeAsynqHandler{}
	mux := buildWorkerMux(operationProcessor, notificationProcessor)

	operationPayload, err := json.Marshal(operationlog.TaskPayload{
		SchemaVersion: 2, EventID: "worker-operation-event", RequestID: "request-1", Method: "PUT", Route: "/api/admin/v1/user/account/:id",
		Module: "user", Action: "user.update", ClientIP: "127.0.0.1", UserAgent: "test",
		StatusCode: 200, IsSuccess: 1, LatencyMs: 1, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(operationlog.TaskType, operationPayload)); err != nil {
		t.Fatalf("process operation log task: %v", err)
	}
	if operationProcessor.processed != "request-1" {
		t.Fatalf("processed operation=%q", operationProcessor.processed)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(scheduler.EnvelopeTaskType, []byte(`{}`))); err != nil {
		t.Fatalf("process scheduler task: %v", err)
	}
	if notificationProcessor.calls != 1 {
		t.Fatalf("notification calls=%d want 1", notificationProcessor.calls)
	}
}

type fakeAsynqHandler struct{ calls int }

func (h *fakeAsynqHandler) ProcessTask(context.Context, *asynq.Task) error { h.calls++; return nil }

type fakeRelayRunner struct {
	mutex       sync.Mutex
	startCount  int
	cancelCount int
	runErr      error
	started     chan struct{}
	stopped     chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

func newFakeRelayRunner() *fakeRelayRunner {
	return &fakeRelayRunner{started: make(chan struct{}), stopped: make(chan struct{})}
}

func (f *fakeRelayRunner) Run(ctx context.Context) error {
	f.mutex.Lock()
	f.startCount++
	f.mutex.Unlock()
	f.startOnce.Do(func() { close(f.started) })
	if f.runErr != nil {
		return f.runErr
	}
	<-ctx.Done()
	f.mutex.Lock()
	f.cancelCount++
	f.mutex.Unlock()
	f.stopOnce.Do(func() { close(f.stopped) })
	return nil
}

func (f *fakeRelayRunner) counts() (int, int) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.startCount, f.cancelCount
}

type fakeAsynqServer struct {
	mutex      sync.Mutex
	startCalls int
	shutdowns  int
	startErr   error
}

func (f *fakeAsynqServer) Start(asynq.Handler) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.startCalls++
	return f.startErr
}

func (f *fakeAsynqServer) Shutdown() {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.shutdowns++
}

func (f *fakeAsynqServer) counts() (int, int) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.startCalls, f.shutdowns
}

func TestWorkerAssemblyStartsRelayThenCancelsOnShutdown(t *testing.T) {
	processContext, cancel := context.WithCancel(context.Background())
	relay := newFakeRelayRunner()
	server := &fakeAsynqServer{}
	done := make(chan error, 1)
	go func() {
		done <- runWorkerAssembly(workerAssembly{
			ProcessContext: processContext,
			Logger:         discardLogger(),
			Mux:            asynq.NewServeMux(),
			Runners:        []namedRunner{{Name: "config-generation", Runner: relay}},
			NewServer:      func() (asynqServer, error) { return server, nil },
		})
	}()

	select {
	case <-relay.started:
	case <-time.After(3 * time.Second):
		t.Fatal("relay did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("assembly error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("assembly did not stop after cancel")
	}
	select {
	case <-relay.stopped:
	case <-time.After(time.Second):
		t.Fatal("relay was not cancelled")
	}
	if starts, cancels := relay.counts(); starts != 1 || cancels != 1 {
		t.Fatalf("relay start/cancel = %d/%d want 1/1", starts, cancels)
	}
	if starts, shutdowns := server.counts(); starts != 1 || shutdowns != 1 {
		t.Fatalf("asynq start/shutdown = %d/%d want 1/1", starts, shutdowns)
	}
}

func TestWorkerAssemblyReportsNamedRunnerFailureAndShutsDownAsynq(t *testing.T) {
	relay := newFakeRelayRunner()
	relay.runErr = errors.New("relay unavailable")
	server := &fakeAsynqServer{}
	err := runWorkerAssembly(workerAssembly{
		ProcessContext: context.Background(),
		Logger:         discardLogger(),
		Mux:            asynq.NewServeMux(),
		Runners:        []namedRunner{{Name: "realtime-outbox", Runner: relay}},
		NewServer:      func() (asynqServer, error) { return server, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "realtime-outbox") {
		t.Fatalf("error = %v want named runner context", err)
	}
	if _, shutdowns := server.counts(); shutdowns != 1 {
		t.Fatalf("shutdowns=%d want 1", shutdowns)
	}
}

func TestWorkerAssemblyCancelsRelayWhenAsynqStartFails(t *testing.T) {
	relay := newFakeRelayRunner()
	server := &fakeAsynqServer{startErr: errors.New("bind failed")}

	err := runWorkerAssembly(workerAssembly{
		ProcessContext: context.Background(),
		Logger:         discardLogger(),
		Mux:            asynq.NewServeMux(),
		Runners:        []namedRunner{{Name: "config-generation", Runner: relay}},
		NewServer:      func() (asynqServer, error) { return server, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "start Asynq Worker") {
		t.Fatalf("error = %v want Asynq start failure", err)
	}
	select {
	case <-relay.stopped:
	case <-time.After(time.Second):
		t.Fatal("relay must be cancelled after a failed Asynq start")
	}
	if _, cancels := relay.counts(); cancels != 1 {
		t.Fatalf("relay cancels = %d want 1", cancels)
	}
	if _, shutdowns := server.counts(); shutdowns != 0 {
		t.Fatalf("failed Asynq start must not call Shutdown: %d", shutdowns)
	}
}

func TestWorkerAssemblyRequiresFactories(t *testing.T) {
	if err := runWorkerAssembly(workerAssembly{ProcessContext: context.Background()}); err == nil {
		t.Fatal("assembly accepted missing factories")
	}
}

func TestWorkerAssemblyStartsAllNamedRunners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runners := []namedRunner{}
	fakes := []*fakeRelayRunner{}
	for _, name := range []string{"config-generation", "realtime-outbox", "scheduler-scanner", "scheduler-publisher"} {
		fake := newFakeRelayRunner()
		fakes = append(fakes, fake)
		runners = append(runners, namedRunner{Name: name, Runner: fake})
	}
	done := make(chan error, 1)
	go func() {
		done <- runWorkerAssembly(workerAssembly{ProcessContext: ctx, Logger: discardLogger(), Mux: asynq.NewServeMux(), Runners: runners, NewServer: func() (asynqServer, error) { return &fakeAsynqServer{}, nil }})
	}()
	for i, fake := range fakes {
		select {
		case <-fake.started:
		case <-time.After(time.Second):
			t.Fatalf("runner %d did not start", i)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
