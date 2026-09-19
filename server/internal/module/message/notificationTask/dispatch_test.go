package notificationtask

import (
	"context"
	"errors"
	"github.com/hibiken/asynq"
	"testing"
	"time"
)

func TestDispatchPayloadStrictDecoder(t *testing.T) {
	valid := []byte(`{"schemaVersion":1,"taskId":42,"batchNo":0}`)
	payload, err := DecodeBatchPayload(valid)
	if err != nil || payload.TaskID != 42 {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}
	for _, raw := range [][]byte{[]byte(`{"schemaVersion":1,"taskId":0,"batchNo":0}`), []byte(`{"schemaVersion":1,"taskId":1,"batchNo":-1}`), []byte(`{"schemaVersion":1,"taskId":1,"batchNo":0,"unknown":1}`), append(valid, []byte(` {}`)...)} {
		if _, err := DecodeBatchPayload(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestDispatchRelayReschedulesFailureAndMarksConflict(t *testing.T) {
	now := time.Now().UTC()
	repo := &dispatchRepositoryStub{rows: []DispatchClaim{{ID: 1, TaskID: 42, BatchNo: 0}, {ID: 2, TaskID: 43, BatchNo: 1}}}
	queue := &dispatchQueueStub{errors: []error{errors.New("redis down"), asynq.ErrTaskIDConflict}}
	relay := NewDispatchRelay(repo, queue, nil)
	relay.now = func() time.Time { return now }
	count, err := relay.RunOnce(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if len(repo.rescheduled) != 1 || len(repo.marked) != 1 || repo.marked[0] != 2 {
		t.Fatalf("rescheduled=%v marked=%v", repo.rescheduled, repo.marked)
	}
}

type dispatchRepositoryStub struct {
	rows        []DispatchClaim
	marked      []int64
	rescheduled []int64
}

func (s *dispatchRepositoryStub) ClaimDispatch(context.Context, int, string, time.Time, time.Duration) ([]DispatchClaim, error) {
	return s.rows, nil
}
func (s *dispatchRepositoryStub) MarkDispatch(_ context.Context, id int64, _ string, _ time.Time) error {
	s.marked = append(s.marked, id)
	return nil
}
func (s *dispatchRepositoryStub) RescheduleDispatch(_ context.Context, id int64, _ string, _ string, _ time.Time, _ time.Time) error {
	s.rescheduled = append(s.rescheduled, id)
	return nil
}

type dispatchQueueStub struct {
	errors []error
	calls  int
}

func (s *dispatchQueueStub) EnqueueBatch(context.Context, BatchPayload) error {
	err := s.errors[s.calls]
	s.calls++
	return err
}
