package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
	var p struct {
		RunID int64 `json:"runId"`
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil || p.RunID < 1 {
		return errors.New("AI run payload is invalid")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("AI run payload is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	row, err := w.Loader.Find(ctx, p.RunID)
	if err != nil {
		return err
	}
	return w.Executor.Execute(ctx, row)
}
