package cos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	tencos "github.com/tencentyun/cos-go-sdk-v5"
)

const (
	PresignValidity    = 10 * time.Minute
	GetPresignValidity = 10 * time.Minute
)

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
	bucket := normalizedBucketName(credentials.Bucket, credentials.AppID)
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

func normalizedBucketName(bucket, appID string) string {
	bucket = strings.TrimSpace(bucket)
	appID = strings.TrimSpace(appID)
	if strings.HasSuffix(bucket, "-"+appID) {
		return bucket
	}
	return bucket + "-" + appID
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
	if request.ContentLength < 1 || !validObjectKey(request.ObjectKey) {
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

func (c *Client) PresignGet(ctx context.Context, credentials Credentials, request GetRequest) (GetResult, error) {
	if !validObjectKey(request.ObjectKey) {
		return GetResult{}, fmt.Errorf("GET request is invalid")
	}
	client, err := c.sdkClient(credentials)
	if err != nil {
		return GetResult{}, err
	}
	now := time.Now().UTC()
	query := &url.Values{}
	if request.ContentDisposition != "" {
		query.Set("response-content-disposition", request.ContentDisposition)
	}
	signed, err := client.Object.GetPresignedURL(ctx, http.MethodGet, request.ObjectKey, credentials.SecretID, credentials.SecretKey, GetPresignValidity, &tencos.PresignedURLOptions{Query: query}, true)
	if err != nil {
		return GetResult{}, err
	}
	return GetResult{URL: signed.String(), ExpiresAt: now.Add(GetPresignValidity)}, nil
}

var ErrObjectNotFound = errors.New("COS object does not exist")
var ErrObjectChanged = errors.New("COS source object changed")

func objectError(err error) error {
	var e *tencos.ErrorResponse
	if errors.As(err, &e) && e.Response != nil {
		if e.Response.StatusCode == 404 {
			return ErrObjectNotFound
		}
		if e.Response.StatusCode == 412 {
			return ErrObjectChanged
		}
	}
	return err
}
func (c *Client) HeadObject(ctx context.Context, credentials Credentials, key string) (ObjectInfo, error) {
	if !validObjectKey(key) {
		return ObjectInfo{}, fmt.Errorf("object key invalid")
	}
	client, err := c.sdkClient(credentials)
	if err != nil {
		return ObjectInfo{}, err
	}
	response, err := client.Object.Head(ctx, key, nil)
	if err != nil {
		return ObjectInfo{}, objectError(err)
	}
	n, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil || n < 1 || response.Header.Get("ETag") == "" {
		return ObjectInfo{}, fmt.Errorf("object HEAD metadata invalid")
	}
	return ObjectInfo{Key: key, MIME: strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])), Size: n, ETag: response.Header.Get("ETag")}, nil
}
func (c *Client) CopyObject(ctx context.Context, source, target Credentials, request CopyRequest) (ObjectInfo, error) {
	if !validObjectKey(request.SourceKey) || !validObjectKey(request.TargetKey) || strings.TrimSpace(request.SourceETag) == "" {
		return ObjectInfo{}, fmt.Errorf("copy request invalid")
	}
	sourceClient, err := c.sdkClient(source)
	if err != nil {
		return ObjectInfo{}, err
	}
	client, err := c.sdkClient(target)
	if err != nil {
		return ObjectInfo{}, err
	}
	headers := http.Header{}
	// COS cross-account copy authenticates its source independently.
	authTime := tencos.NewAuthTime(time.Minute * 5)
	sourceURL := *sourceClient.BaseURL.BucketURL
	sourceURL.Path = "/" + request.SourceKey
	sourceRequest := &http.Request{Method: http.MethodGet, URL: &sourceURL, Header: http.Header{}}
	tencos.AddAuthorizationHeader(source.SecretID, source.SecretKey, "", sourceRequest, authTime)
	headers.Set("x-cos-copy-source-authorization", sourceRequest.Header.Get("Authorization"))
	options := &tencos.ObjectCopyOptions{ObjectCopyHeaderOptions: &tencos.ObjectCopyHeaderOptions{XCosCopySourceIfMatch: request.SourceETag, XCosMetadataDirective: "Copy", XOptionHeader: &headers}, ACLHeaderOptions: &tencos.ACLHeaderOptions{XCosACL: "private"}}
	if request.PublicRead {
		options.ACLHeaderOptions.XCosACL = "public-read"
	}
	_, _, err = client.Object.Copy(ctx, request.TargetKey, sourceURL.Host+"/"+request.SourceKey, options)
	if err != nil {
		return ObjectInfo{}, objectError(err)
	}
	return c.HeadObject(ctx, target, request.TargetKey)
}
func (c *Client) DeleteObject(ctx context.Context, credentials Credentials, key string) error {
	if !validObjectKey(key) {
		return fmt.Errorf("object key invalid")
	}
	client, err := c.sdkClient(credentials)
	if err != nil {
		return err
	}
	_, err = client.Object.Delete(ctx, key)
	err = objectError(err)
	if errors.Is(err, ErrObjectNotFound) {
		return nil
	}
	return err
}
func (c *Client) ReadObject(ctx context.Context, credentials Credentials, key, etag string, limit int64) ([]byte, error) {
	if !validObjectKey(key) || etag == "" || limit < 1 {
		return nil, fmt.Errorf("read request invalid")
	}
	client, err := c.sdkClient(credentials)
	if err != nil {
		return nil, err
	}
	headers := http.Header{"If-Match": []string{etag}}
	response, err := client.Object.Get(ctx, key, &tencos.ObjectGetOptions{XOptionHeader: &headers})
	if err != nil {
		return nil, objectError(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("object exceeds read limit")
	}
	return data, nil
}

func validObjectKey(objectKey string) bool {
	return strings.TrimSpace(objectKey) != "" && !strings.HasPrefix(objectKey, "/") &&
		!strings.Contains(objectKey, "..") && !strings.Contains(objectKey, "\\") &&
		strings.IndexFunc(objectKey, unicode.IsControl) < 0
}
