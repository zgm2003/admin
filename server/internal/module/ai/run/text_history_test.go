package run

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fixtureTextFiles struct{ calls int }

func (f *fixtureTextFiles) ReadAIFrozenReference(context.Context, int64, int64, InputFileSnapshot, int64) ([]byte, string, string, error) {
	f.calls++
	return []byte{137, 80, 78, 71}, "image/png", "fixture.png", nil
}
func TestTextInputRestoresRolesAndInlineImagesWithoutSummaryLeak(t *testing.T) {
	reader := &fixtureTextFiles{}
	e := &TextEngine{}
	e.SetFileReader(reader)
	policy := Policy{MaxInputAttachments: 4, ImageMaxBytes: 100, MaxTextInputBytes: 1000}
	history := []historyTurn{{Role: "user", Content: json.RawMessage(`[{"type":"input_text","text":"first"}]`)}, {Role: "assistant", Content: json.RawMessage(`[{"type":"output_text","text":"answer"}]`)}}
	input := RunInput{Text: "next", FileSnapshots: []InputFileSnapshot{{ID: "1", MIMEType: "image/png", SizeBytes: 4}}}
	native, err := e.prepareTextInput(context.Background(), Model{UserID: 1, PlatformID: 2}, 1, history, input, policy)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(native)
	if len(native) != 3 || !strings.Contains(string(b), "data:image/png;base64,") || reader.calls != 1 {
		t.Fatalf("%s calls=%d", b, reader.calls)
	}
	summary, err := textRequestSummary(native)
	if err != nil || strings.Contains(string(summary), "base64") || strings.Contains(string(summary), "iVBORw") {
		t.Fatalf("%s %v", summary, err)
	}
	count, err := nativeContextBytes(native)
	if err != nil || count > 200 {
		t.Fatalf("context counts base64: %d %v", count, err)
	}
}
func TestTextInputRejectsTooManyHistoricalAttachments(t *testing.T) {
	e := &TextEngine{}
	input := RunInput{Text: "next", FileSnapshots: make([]InputFileSnapshot, 5)}
	if _, err := e.prepareTextInput(context.Background(), Model{}, 2, nil, input, Policy{MaxInputAttachments: 4}); err == nil {
		t.Fatal("unbounded attachments accepted")
	}
}
