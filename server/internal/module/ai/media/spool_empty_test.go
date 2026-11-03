package media

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSpoolEmptyIsRejected(t *testing.T) {
	s, e := NewSpooler(t.TempDir(), 10)
	if e != nil {
		t.Fatal(e)
	}
	f, e := s.FromBase64(context.Background(), io.NopCloser(strings.NewReader("")), Constraints{MaxBytes: 10, AllowedMIMEs: []string{"image/png"}})
	if e == nil {
		f.Close()
		t.Fatal("empty base64 accepted")
	}
}
