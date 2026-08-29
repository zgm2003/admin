package cosconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	IsEnabled *yesno.Value
}
type SecretInput struct {
	Present bool
	Value   string
}
type CreateInput struct {
	Name, AppID, SecretID, SecretKey, Bucket, Region string
	Endpoint, BucketDomain                           *string
	IsEnabled                                        yesno.Value
	Remark                                           string
}
type UpdateInput struct {
	Name, AppID, Bucket, Region string
	Endpoint, BucketDomain      *string
	SecretID, SecretKey         SecretInput
	Remark                      string
}

type createRequest struct {
	Name         *string               `json:"name"`
	AppID        *string               `json:"appId"`
	SecretID     *string               `json:"secretId"`
	SecretKey    *string               `json:"secretKey"`
	Bucket       *string               `json:"bucket"`
	Region       *string               `json:"region"`
	Endpoint     nullableStringRequest `json:"endpoint"`
	BucketDomain nullableStringRequest `json:"bucketDomain"`
	IsEnabled    *yesno.Value          `json:"isEnabled"`
	Remark       *string               `json:"remark"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Name == nil || r.AppID == nil || r.SecretID == nil || r.SecretKey == nil || r.Bucket == nil || r.Region == nil || r.IsEnabled == nil || r.Remark == nil {
		return CreateInput{}, fmt.Errorf("every COS config field is required")
	}
	input := CreateInput{Name: *r.Name, AppID: *r.AppID, SecretID: *r.SecretID, SecretKey: *r.SecretKey, Bucket: *r.Bucket, Region: *r.Region, Endpoint: r.Endpoint.Value, BucketDomain: r.BucketDomain.Value, IsEnabled: *r.IsEnabled, Remark: *r.Remark}
	if err := validateCreate(input); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

type updateRequest struct {
	Name         *string               `json:"name"`
	AppID        *string               `json:"appId"`
	SecretID     json.RawMessage       `json:"secretId"`
	SecretKey    json.RawMessage       `json:"secretKey"`
	Bucket       *string               `json:"bucket"`
	Region       *string               `json:"region"`
	Endpoint     nullableStringRequest `json:"endpoint"`
	BucketDomain nullableStringRequest `json:"bucketDomain"`
	Remark       *string               `json:"remark"`
}

func parseSecretInput(raw json.RawMessage) (SecretInput, error) {
	if raw == nil {
		return SecretInput{}, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return SecretInput{}, fmt.Errorf("secret replacement cannot be null")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return SecretInput{}, fmt.Errorf("secret replacement is invalid")
	}
	return SecretInput{Present: true, Value: value}, nil
}
func (r updateRequest) input() (UpdateInput, error) {
	if r.Name == nil || r.AppID == nil || r.Bucket == nil || r.Region == nil || r.Remark == nil {
		return UpdateInput{}, fmt.Errorf("every COS config metadata field is required")
	}
	sid, e := parseSecretInput(r.SecretID)
	if e != nil {
		return UpdateInput{}, e
	}
	skey, e := parseSecretInput(r.SecretKey)
	if e != nil {
		return UpdateInput{}, e
	}
	input := UpdateInput{Name: *r.Name, AppID: *r.AppID, Bucket: *r.Bucket, Region: *r.Region, Endpoint: r.Endpoint.Value, BucketDomain: r.BucketDomain.Value, SecretID: sid, SecretKey: skey, Remark: *r.Remark}
	if err := validateUpdate(input); err != nil {
		return UpdateInput{}, err
	}
	return input, nil
}

type nullableStringRequest struct {
	Value *string
}

func (r *nullableStringRequest) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		r.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	r.Value = &value
	return nil
}

type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func parseListQuery(values url.Values) (ListQuery, error) {
	allowed := map[string]struct{}{"page": {}, "pageSize": {}, "keyword": {}, "isEnabled": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("invalid list query")
		}
	}
	page, e := strconv.Atoi(values.Get("page"))
	if e != nil || page < 1 {
		return ListQuery{}, fmt.Errorf("page invalid")
	}
	size, e := strconv.Atoi(values.Get("pageSize"))
	if e != nil || size < 1 || size > 100 {
		return ListQuery{}, fmt.Errorf("pageSize invalid")
	}
	q := ListQuery{Page: page, PageSize: size, Keyword: values.Get("keyword")}
	if raw := values.Get("isEnabled"); raw != "" {
		n, e := strconv.Atoi(raw)
		v := yesno.Value(n)
		if e != nil || !yesno.IsValid(v) {
			return ListQuery{}, fmt.Errorf("isEnabled invalid")
		}
		q.IsEnabled = &v
	}
	return q, nil
}

func validateText(name, value string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > max {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}
func validateCreate(input CreateInput) error {
	for _, v := range []struct {
		n, s string
		m    int
	}{{"name", input.Name, 128}, {"appID", input.AppID, 32}, {"secretID", input.SecretID, 128}, {"secretKey", input.SecretKey, 256}, {"bucket", input.Bucket, 128}, {"region", input.Region, 64}} {
		if err := validateText(v.n, v.s, v.m); err != nil {
			return err
		}
	}
	if !yesno.IsValid(input.IsEnabled) {
		return fmt.Errorf("isEnabled is invalid")
	}
	if err := validateOptionalURL("endpoint", input.Endpoint); err != nil {
		return err
	}
	if err := validateOptionalURL("bucketDomain", input.BucketDomain); err != nil {
		return err
	}
	if len(strings.TrimSpace(input.Remark)) > 512 {
		return fmt.Errorf("remark is invalid")
	}
	return nil
}
func validateUpdate(input UpdateInput) error {
	if err := validateText("name", input.Name, 128); err != nil {
		return err
	}
	if err := validateText("appID", input.AppID, 32); err != nil {
		return err
	}
	if err := validateText("bucket", input.Bucket, 128); err != nil {
		return err
	}
	if err := validateText("region", input.Region, 64); err != nil {
		return err
	}
	if input.SecretID.Present && strings.TrimSpace(input.SecretID.Value) == "" {
		return fmt.Errorf("secretId must not be empty")
	}
	if input.SecretKey.Present && strings.TrimSpace(input.SecretKey.Value) == "" {
		return fmt.Errorf("secretKey must not be empty")
	}
	if err := validateOptionalURL("endpoint", input.Endpoint); err != nil {
		return err
	}
	if err := validateOptionalURL("bucketDomain", input.BucketDomain); err != nil {
		return err
	}
	if len(strings.TrimSpace(input.Remark)) > 512 {
		return fmt.Errorf("remark is invalid")
	}
	return nil
}

func validateOptionalURL(field string, value *string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(*value))
	if err != nil || u.Scheme != "https" || u.Host == "" || !validHostname(u.Hostname()) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("%s is invalid", field)
	}
	return nil
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}
func invalid(err error) error { return apperror.InvalidRequest(err) }
