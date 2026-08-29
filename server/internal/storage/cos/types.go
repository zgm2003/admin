package cos

import "context"

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
type Presigner interface {
	PresignPut(context.Context, Credentials, PutRequest) (PutResult, error)
}
