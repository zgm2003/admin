package cos

import (
	"context"
	"io"
	"time"
)

type Credentials struct {
	AppID     string
	SecretID  string
	SecretKey string
	Bucket    string
	Region    string
	Endpoint  string
}

type ConnectionTester interface {
	TestConnection(context.Context, Credentials) error
}

type PutRequest struct {
	ObjectKey     string
	ContentType   string
	ContentLength int64
	PublicRead    bool
}
type PutResult struct {
	URL     string
	Headers map[string]string
}
type GetRequest struct {
	ObjectKey          string
	ContentDisposition string
}
type GetResult struct {
	URL       string
	ExpiresAt time.Time
}
type Presigner interface {
	PresignPut(context.Context, Credentials, PutRequest) (PutResult, error)
	PresignGet(context.Context, Credentials, GetRequest) (GetResult, error)
}

type ObjectInfo struct {
	Key, MIME, ETag string
	Size            int64
}
type CopyRequest struct {
	SourceKey, TargetKey, SourceETag string
	PublicRead                       bool
}
type ObjectClient interface {
	Presigner
	HeadObject(context.Context, Credentials, string) (ObjectInfo, error)
	CopyObject(context.Context, Credentials, Credentials, CopyRequest) (ObjectInfo, error)
	DeleteObject(context.Context, Credentials, string) error
	ReadObject(context.Context, Credentials, string, string, int64) ([]byte, error)
}

// StreamPutter is an optional extension used by server-side generated-media
// imports. It deliberately remains separate from ObjectClient so existing
// upload/test doubles do not need to implement a write path.
type StreamPutter interface {
	PutStream(context.Context, Credentials, PutStreamRequest) (ObjectInfo, error)
}

type PutStreamRequest struct {
	ObjectKey     string
	ContentType   string
	ContentLength int64
	Body          io.Reader
	PublicRead    bool
}
