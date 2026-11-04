package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"time"
)

type Executor interface {
	Execute(context.Context, Model) error
}
type Loader interface {
	Find(context.Context, int64) (Model, error)
}
type Worker struct {
	Loader   Loader
	Executor Executor
}

func (w *Worker) Execute(ctx context.Context, payload json.RawMessage) error {
	if w == nil || w.Loader == nil || w.Executor == nil {
		return errors.New("AI run worker is not configured")
	}
	runID, err := parseRunID(payload)
	if err != nil {
		return errors.New("AI run payload is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	row, err := w.Loader.Find(ctx, runID)
	if err != nil {
		return err
	}
	return w.Executor.Execute(ctx, row)
}

func (w *Worker) TimeoutForPayload(ctx context.Context, payload json.RawMessage, now time.Time) (time.Duration, error) {
	const finishWindow = 30 * time.Second
	if w == nil || w.Loader == nil {
		return 0, errors.New("AI run worker is not configured")
	}
	runID, err := parseRunID(payload)
	if err != nil {
		return finishWindow, nil
	}
	row, err := w.Loader.Find(ctx, runID)
	if err != nil {
		return 0, err
	}
	if row.ExecutionStatus != 0 || row.DeadlineAt.IsZero() || !row.DeadlineAt.After(now) {
		return finishWindow, nil
	}
	var transfer time.Duration
	switch row.Operation {
	case "image.generate", "image.edit", "audio.speech", "audio.transcribe", "video.generate":
		var policy struct {
			TransferTimeoutSeconds int64 `json:"transferTimeoutSeconds"`
		}
		if json.Unmarshal(row.PolicySnapshot, &policy) == nil && policy.TransferTimeoutSeconds > 0 {
			transfer = durationFromSeconds(policy.TransferTimeoutSeconds)
			if row.Operation == "image.generate" || row.Operation == "image.edit" {
				var input struct {
					Count int64 `json:"count"`
				}
				if json.Unmarshal(row.InputSnapshot, &input) == nil && input.Count > 1 {
					transfer = multiplyDuration(transfer, input.Count)
				}
			}
		}
	}
	return remainingBudget(row.DeadlineAt, now, transfer, finishWindow), nil
}

func durationFromSeconds(seconds int64) time.Duration {
	const maxDuration = time.Duration(math.MaxInt64)
	if seconds <= 0 || seconds > math.MaxInt64/int64(time.Second) {
		return maxDuration
	}
	return time.Duration(seconds) * time.Second
}

func multiplyDuration(value time.Duration, multiplier int64) time.Duration {
	const maxDuration = time.Duration(math.MaxInt64)
	if value <= 0 || multiplier <= 0 || value > maxDuration/time.Duration(multiplier) {
		return maxDuration
	}
	return value * time.Duration(multiplier)
}

func parseRunID(payload json.RawMessage) (int64, error) {
	var p struct {
		RunID int64 `json:"runId"`
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil || p.RunID < 1 {
		return 0, errors.New("AI run payload is invalid")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return 0, errors.New("AI run payload is invalid")
	}
	return p.RunID, nil
}

func remainingBudget(deadline time.Time, now time.Time, extension, finishWindow time.Duration) time.Duration {
	if !deadline.After(now) {
		return finishWindow
	}
	remaining := deadline.Sub(now)
	if extension > 0 {
		if remaining > time.Duration(math.MaxInt64)-extension {
			return time.Duration(math.MaxInt64)
		}
		remaining += extension
	}
	if remaining > time.Duration(math.MaxInt64)-finishWindow {
		return time.Duration(math.MaxInt64)
	}
	return remaining + finishWindow
}
