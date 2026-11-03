package media

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSpoolerEnforcesQuotaAndDeletesOnClose(t *testing.T) {
	s, err := NewSpooler(t.TempDir(), 10)
	if err != nil {
		t.Fatal(err)
	}
	png := "\x89PNG\r\n\x1a\nab"
	f, err := s.FromStream(context.Background(), io.NopCloser(strings.NewReader(png)), "image/png", int64(len(png)), Constraints{MaxBytes: 16, AllowedMIMEs: []string{"image/png"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.SizeBytes != int64(len(png)) || f.SHA256 == "" {
		t.Fatalf("%+v", f)
	}
	if _, err = s.FromStream(context.Background(), io.NopCloser(strings.NewReader("x")), "image/png", 1, Constraints{MaxBytes: 16, AllowedMIMEs: []string{"image/png"}}); err == nil {
		t.Fatal("quota was not enforced")
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSpoolerRejectsTooLargeAndDisallowedMime(t *testing.T) {
	s, _ := NewSpooler(t.TempDir(), 100)
	if _, err := s.FromStream(context.Background(), io.NopCloser(strings.NewReader("abc")), "text/html", 3, Constraints{MaxBytes: 10, AllowedMIMEs: []string{"image/png"}}); err == nil {
		t.Fatal("mime accepted")
	}
	if _, err := s.FromStream(context.Background(), io.NopCloser(strings.NewReader("abcdef")), "image/png", 6, Constraints{MaxBytes: 5, AllowedMIMEs: []string{"image/png"}}); err == nil {
		t.Fatal("size accepted")
	}
}

func TestSpoolerChecksFragmentedSignature(t *testing.T) {
	s, e := NewSpooler(t.TempDir(), 100)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		raw string
		ok  bool
	}{{"abcdefgh", false}, {"\x89PNG\r\n\x1a\n", true}, {"x", false}} {
		f, e := s.FromStream(context.Background(), &fragmentReader{Reader: strings.NewReader(tc.raw)}, "image/png", int64(len(tc.raw)), Constraints{MaxBytes: 20, AllowedMIMEs: []string{"image/png"}})
		if (e == nil) != tc.ok {
			t.Errorf("raw=%q err=%v", tc.raw, e)
		}
		if f != nil {
			f.Close()
		}
	}
}

type fragmentReader struct{ io.Reader }

func (f *fragmentReader) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return f.Reader.Read(b)
}
func (f *fragmentReader) Close() error { return nil }
