package providerrequest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestImagesGenerateSendsNativePayloadAndParsesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" || r.Method != http.MethodPost {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("x-request-id", "req-1")
		w.Header().Set("content-type", "application/json")
		io.WriteString(w, `{"created":1,"data":[{"url":"https://cdn.example/x.png","revised_prompt":"ok"}]}`)
	}))
	defer srv.Close()
	c := NewImagesClient(srv.Client())
	got, err := c.Generate(context.Background(), ImageRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "image-model", Timeout: time.Second}, Prompt: "draw", N: 1, Size: "1024x1024", Quality: "high", OutputFormat: "png"})
	if err != nil || len(got.Items) != 1 || got.Items[0].URL != "https://cdn.example/x.png" || got.RequestID != "req-1" {
		t.Fatalf("got %#v err %v", got, err)
	}
}

func TestVideoSubmitDoesNotRetryUnknownPost(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { attempts++; http.Error(w, "oops", http.StatusBadGateway) }))
	defer srv.Close()
	c := NewVideoClient(srv.Client())
	_, err := c.Submit(context.Background(), VideoRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "video", Timeout: time.Second}, Prompt: "x"})
	if err == nil || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestImagesEditSendsMultipartImageAndFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" || r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("request=%s %s content-type=%q", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != "image-model" || r.FormValue("prompt") != "edit" || r.FormValue("n") != "1" || r.FormValue("size") != "1024x1024" {
			t.Fatalf("fields=%v", r.MultipartForm.Value)
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if header.Filename != "input.png" {
			t.Fatalf("filename=%q", header.Filename)
		}
		data, _ := io.ReadAll(file)
		if string(data) != "\x89PNG\r\n\x1a\n" {
			t.Fatalf("image=%q", data)
		}
		w.Header().Set("x-request-id", "edit-1")
		io.WriteString(w, `{"created":1,"data":[{"b64_json":"aGVsbG8="}]}`)
	}))
	defer srv.Close()
	got, err := NewImagesClient(srv.Client()).Edit(context.Background(), ImageEditRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "image-model", Timeout: time.Second}, Prompt: "edit", N: 1, Size: "1024x1024", Image: strings.NewReader("\x89PNG\r\n\x1a\n"), Filename: "input.png", MIME: "image/png"})
	if err != nil || len(got.Items) != 1 || got.Items[0].B64JSON != "aGVsbG8=" || got.RequestID != "edit-1" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestAudioTranscribeSendsMultipartAudio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" || r.Method != http.MethodPost {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "input.mp3" || string(data) != "audio" || r.FormValue("model") != "audio-model" || r.FormValue("response_format") != "json" || r.FormValue("prompt") != "hint" {
			t.Fatalf("filename=%q data=%q values=%v", header.Filename, data, r.MultipartForm.Value)
		}
		w.Header().Set("x-request-id", "transcribe-1")
		io.WriteString(w, `{"text":"hello","duration":2.5}`)
	}))
	defer srv.Close()
	got, err := NewAudioClient(srv.Client()).Transcribe(context.Background(), TranscriptionRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "audio-model", Timeout: time.Second}, Audio: strings.NewReader("audio"), Filename: "input.mp3", MIME: "audio/mpeg", ResponseFormat: "json", Prompt: "hint"})
	if err != nil || got.Text != "hello" || got.RequestID != "transcribe-1" || !got.Usage.Known || got.Usage.DurationSeconds != "2.5" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestMediaMultipartPreservesMIMEAndParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Error(err)
			http.Error(w, "bad", 400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		field := "image"
		if strings.Contains(r.URL.Path, "transcriptions") {
			field = "file"
		}
		f, h, e := r.FormFile(field)
		if e != nil {
			t.Error(e)
			return
		}
		f.Close()
		if h.Header.Get("Content-Type") != "image/png" || r.FormValue("temperature") != "0.2" {
			t.Errorf("mime=%q params=%v", h.Header.Get("Content-Type"), r.MultipartForm.Value)
		}
		if field == "image" {
			io.WriteString(w, `{"data":[{"url":"https://HOST/x"}]}`)
		} else {
			io.WriteString(w, `{"text":"x"}`)
		}
	}))
	defer srv.Close()
	cfg := FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second}
	_, e := NewImagesClient(srv.Client()).Edit(context.Background(), ImageEditRequest{Config: cfg, Prompt: "x", N: 1, Image: strings.NewReader("png"), MIME: "image/png", Params: []byte(`{"temperature":0.2}`)})
	if e != nil {
		t.Fatal(e)
	}
	_, e = NewAudioClient(srv.Client()).Transcribe(context.Background(), TranscriptionRequest{Config: cfg, Audio: strings.NewReader("png"), MIME: "image/png", Params: []byte(`{"temperature":0.2}`)})
	if e != nil {
		t.Fatal(e)
	}
}
func TestImageEditDoesNotSilentlyTruncate(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{"data":[{"url":"https://HOST/x"}]}`)
	}))
	defer srv.Close()
	_, err := NewImagesClient(srv.Client()).Edit(context.Background(), ImageEditRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second}, Prompt: "x", N: 1, Image: strings.NewReader(strings.Repeat("x", (10<<20)+1)), MIME: "image/png"})
	if err == nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestSpeechForwardsInstructionsAndParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), `"instructions":"calm"`) || !strings.Contains(string(raw), `"speed":1.2`) {
			t.Errorf("body=%s", raw)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		io.WriteString(w, "ID3")
	}))
	defer srv.Close()
	out, err := NewAudioClient(srv.Client()).Synthesize(context.Background(), SpeechRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second}, Input: "x", Voice: "v", Instructions: "calm", Params: []byte(`{"speed":1.2}`)})
	if err != nil {
		t.Fatal(err)
	}
	out.Body.Close()
}
