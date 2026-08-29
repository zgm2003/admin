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
