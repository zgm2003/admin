package run

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTextDeltaBatchBoundsSizeFrequencyAndUnicode(t *testing.T) {
	var chunks []string
	var at []time.Time
	batch := newTextDeltaBatch(context.Background(), func(_ context.Context, text string) error {
		chunks = append(chunks, text)
		at = append(at, time.Now())
		return nil
	}, streamPressureFunc(func(context.Context) (int64, error) { return 0, nil }))
	original := strings.Repeat("汉", 6000)
	if e := batch.Append(original); e != nil {
		t.Fatal(e)
	}
	if e := batch.Flush(); e != nil {
		t.Fatal(e)
	}
	if strings.Join(chunks, "") != original {
		t.Fatal("stream text lost")
	}
	for i, chunk := range chunks {
		if len(chunk) > 8192 || !utf8.ValidString(chunk) {
			t.Fatalf("invalid chunk bytes=%d", len(chunk))
		}
		if i > 0 && at[i].Sub(at[i-1]) < textDeltaInterval-10*time.Millisecond {
			t.Fatal("event frequency exceeds bound")
		}
	}
}
func TestTextDeltaBatchCancellationStopsBackpressure(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	batch := newTextDeltaBatch(ctx, func(context.Context, string) error { return nil }, streamPressureFunc(func(context.Context) (int64, error) { return 0, nil }))
	if e := batch.Append(strings.Repeat("a", 8192)); e != nil {
		t.Fatal(e)
	}
	c()
	if e := batch.Append(strings.Repeat("b", 8192)); e == nil {
		t.Fatal("canceled stream accepted")
	}
}
