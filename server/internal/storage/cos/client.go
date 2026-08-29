package cos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	tencos "github.com/tencentyun/cos-go-sdk-v5"
)

const PresignValidity = 10 * time.Minute

type Client struct{ httpClient *http.Client }

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient}
}
func (c *Client) sdkClient(credentials Credentials) (*tencos.Client, error) {
	if strings.TrimSpace(credentials.Bucket) == "" || strings.TrimSpace(credentials.AppID) == "" || strings.TrimSpace(credentials.Region) == "" || strings.TrimSpace(credentials.SecretID) == "" || strings.TrimSpace(credentials.SecretKey) == "" {
		return nil, fmt.Errorf("COS credentials are incomplete")
	}
	bucket := strings.TrimSpace(credentials.Bucket) + "-" + strings.TrimSpace(credentials.AppID)
	var base *tencos.BaseURL
	if strings.TrimSpace(credentials.Endpoint) != "" {
		endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(credentials.Endpoint), "/"))
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			return nil, fmt.Errorf("COS endpoint is invalid")
		}
		base = &tencos.BaseURL{BucketURL: endpoint, ServiceURL: endpoint}
	} else {
		endpoint, err := tencos.NewBucketURL(bucket, strings.TrimSpace(credentials.Region), true)
		if err != nil {
			return nil, err
		}
		base = &tencos.BaseURL{BucketURL: endpoint, ServiceURL: endpoint}
	}
	httpClient := *c.httpClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = &tencos.AuthorizationTransport{SecretID: credentials.SecretID, SecretKey: credentials.SecretKey, Transport: transport}
	client := tencos.NewClient(base, &httpClient)
	client.BaseURL.BucketURL.Path = ""
	return client, nil
}
func (c *Client) TestConnection(ctx context.Context, credentials Credentials) error {
	client, err := c.sdkClient(credentials)
	if err != nil {
		return err
	}
	_, _, err = client.Bucket.Get(ctx, &tencos.BucketGetOptions{MaxKeys: 1})
	return err
}
func (c *Client) PresignPut(ctx context.Context, credentials Credentials, request PutRequest) (PutResult, error) {
	if request.ContentLength < 1 || strings.TrimSpace(request.ObjectKey) == "" || strings.HasPrefix(request.ObjectKey, "/") || strings.Contains(request.ObjectKey, "..") || strings.Contains(request.ObjectKey, "\\") || strings.IndexFunc(request.ObjectKey, unicode.IsControl) >= 0 {
		return PutResult{}, fmt.Errorf("PUT request is invalid")
	}
	client, err := c.sdkClient(credentials)
	if err != nil {
		return PutResult{}, err
	}
	headers := http.Header{}
	if request.ContentType != "" {
		headers.Set("Content-Type", request.ContentType)
	}
	headers.Set("Content-Length", fmt.Sprintf("%d", request.ContentLength))
	if request.PublicRead {
		headers.Set("x-cos-acl", "public-read")
	}
	opt := &tencos.PresignedURLOptions{Header: &headers}
	signed, err := client.Object.GetPresignedURL(ctx, http.MethodPut, request.ObjectKey, credentials.SecretID, credentials.SecretKey, PresignValidity, opt, true)
	if err != nil {
		return PutResult{}, err
	}
	out := map[string]string{}
	for key, values := range headers {
		if len(values) > 0 {
			outputKey := key
			if strings.EqualFold(key, "x-cos-acl") {
				outputKey = "x-cos-acl"
			}
			out[outputKey] = values[0]
		}
	}
	return PutResult{URL: signed.String(), Headers: out}, nil
}
