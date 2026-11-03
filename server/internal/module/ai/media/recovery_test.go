package media

import (
	"context"
	"encoding/base64"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMediaConstraintsPreserveFrozenDeadline(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	deadline := now.Add(time.Second)
	mime := "image/png"
	c, err := mediaConstraints(File{MIMEType: &mime, TransferDeadline: &deadline}, validMediaPolicy(), now)
	if err != nil || !c.Deadline.Equal(deadline) {
		t.Fatalf("deadline=%v err=%v", c.Deadline, err)
	}
}
func TestImportRetryScheduleIsBounded(t *testing.T) {
	if importBackoff(1) != 5*time.Second || importBackoff(2) != 30*time.Second {
		t.Fatal("wrong retry schedule")
	}
}
func TestBase64SpoolRejectsMalformedOversizeAndEmpty(t *testing.T) {
	s, err := NewSpooler(t.TempDir(), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "%%%", base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nextra"))} {
		if f, err := s.FromBase64(context.Background(), io.NopCloser(strings.NewReader(raw)), Constraints{MaxBytes: 8, AllowedMIMEs: []string{"image/png"}, SourceMIME: "image/png"}); err == nil {
			f.Close()
			t.Fatalf("accepted %q", raw)
		}
		if s.used != 0 || len(s.slots) != 0 {
			t.Fatalf("leak used=%d slots=%d", s.used, len(s.slots))
		}
	}
}
func TestBase64SpoolStreamsDecodedBytes(t *testing.T) {
	s, err := NewSpooler(t.TempDir(), 100)
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n"))
	f, err := s.FromBase64(context.Background(), io.NopCloser(strings.NewReader(raw)), Constraints{MaxBytes: 8, AllowedMIMEs: []string{"image/png"}, SourceMIME: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	if f.SizeBytes != 8 {
		t.Fatal(f.SizeBytes)
	}
	f.Close()
	if s.used != 0 {
		t.Fatal(s.used)
	}
}
