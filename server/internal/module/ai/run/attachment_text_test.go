package run

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadAttachmentText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("\xEF\xBB\xBFhello 世界"))
	}))
	defer server.Close()

	text, err := readAttachmentText(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello 世界" {
		t.Fatalf("text = %q", text)
	}
}

func TestReadAttachmentTextRejectsNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	if _, err := readAttachmentText(context.Background(), server.URL); err == nil {
		t.Fatal("expected download error")
	}
}

func TestIsTextAttachment(t *testing.T) {
	cases := []struct {
		mimeType string
		fileName string
		want     bool
	}{
		{"text/plain", "note.txt", true},
		{"application/json", "data.json", true},
		{"", "README.md", true},
		{"application/octet-stream", "sample.TXT", true},
		{"image/png", "logo.png", false},
		{"application/pdf", "report.pdf", false},
		{"application/octet-stream", "archive.zip", false},
	}
	for _, test := range cases {
		if got := isTextAttachment(test.mimeType, test.fileName); got != test.want {
			t.Errorf("isTextAttachment(%q, %q) = %v, want %v", test.mimeType, test.fileName, got, test.want)
		}
	}
}

func TestReadAttachmentTextTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", maxAttachmentTextBytes+1)))
	}))
	defer server.Close()

	text, err := readAttachmentText(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, "[附件内容已截断]") {
		t.Fatal("expected truncation marker")
	}
}
