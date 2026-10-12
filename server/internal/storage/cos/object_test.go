package cos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestObjectOperationsBindConditionAndHandleNotFound(t *testing.T) {
	credentials := Credentials{AppID: "1", SecretID: "sid", SecretKey: "key", Bucket: "assets", Region: "ap-guangzhou", Endpoint: "https://cos.example.com"}
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		h := http.Header{"Content-Length": []string{"4"}, "Content-Type": []string{"image/png"}, "Etag": []string{`"fixed"`}}
		if r.Method == http.MethodPut {
			if r.Header.Get("x-cos-copy-source-If-Match") != `"fixed"` {
				t.Error("copy is not conditioned")
			}
			return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(`<CopyObjectResult><ETag>"fixed"</ETag></CopyObjectResult>`))}, nil
		}
		if r.Method == http.MethodDelete {
			return &http.Response{StatusCode: 404, Header: h, Body: io.NopCloser(strings.NewReader(`<Error><Code>NoSuchKey</Code></Error>`))}, nil
		}
		return &http.Response{StatusCode: 200, Header: h, Body: http.NoBody}, nil
	})})
	info, err := client.HeadObject(context.Background(), credentials, "source.png")
	if err != nil || info.Size != 4 || info.ETag != `"fixed"` {
		t.Fatalf("head: %+v %v", info, err)
	}
	_, err = client.CopyObject(context.Background(), credentials, credentials, CopyRequest{SourceKey: "source.png", TargetKey: "target.png", SourceETag: info.ETag})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.DeleteObject(context.Background(), credentials, "missing.png"); err != nil {
		t.Fatal(err)
	}
	if _, err = client.CopyObject(context.Background(), credentials, credentials, CopyRequest{SourceKey: "a", TargetKey: "b"}); err == nil {
		t.Fatal("copy accepted no etag")
	}
	_ = errors.Is(err, ErrObjectNotFound)
}
