package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxSnapshotBytes   = 256 * 1024
	NotifyPath         = "/api/admin/v1/payment/callback/alipay"
	NotifyPathTemplate = "/api/admin/v1/payment/callback/alipay/{configId}/{version}"
)

var errNotifyURLInvalid = errors.New("notifyUrl must use HTTPS and the exact callback path")

type CreateInput struct {
	Name, AppID, SellerID                                         string
	Provider                                                      payment.Provider
	Environment                                                   payment.Environment
	PrivateKey, AppCertPEM, AlipayCertPEM, RootCertPEM, NotifyURL string
	PayMethods                                                    []payment.PayMethod
	SortOrder                                                     int32
	Status                                                        yesno.Value
}
type MetadataInput struct {
	Name      string
	SortOrder int32
}
type VersionInput struct {
	PrivateKey, AppCertPEM, AlipayCertPEM, RootCertPEM *string
	NotifyURL                                          string
	PayMethods                                         []payment.PayMethod
}
type ListQuery struct {
	Page, PageSize int
	Keyword        string
	Status         *yesno.Value
}

var identityPattern = regexp.MustCompile(`^[0-9]{1,64}$`)

func validateMetadata(v MetadataInput) error {
	if strings.TrimSpace(v.Name) == "" || utf8.RuneCountInString(v.Name) > 128 {
		return fmt.Errorf("name invalid")
	}
	return nil
}
func validateCreate(v CreateInput) error {
	if !v.Provider.Supported() || !v.Environment.Valid() || !yesno.IsValid(v.Status) {
		return fmt.Errorf("configuration identity or flags invalid")
	}
	if err := validateMetadata(MetadataInput{v.Name, v.SortOrder}); err != nil {
		return err
	}
	if _, err := payment.NormalizePayMethods(v.PayMethods); err != nil {
		return err
	}
	_, err := resolveNotifyURL(v.NotifyURL, 1, 1)
	return err
}
func resolveNotifyURL(template string, id, version int64) (string, error) {
	if id < 1 || version < 1 || len(template) > 2048 || strings.TrimSpace(template) != template {
		return "", errNotifyURLInvalid
	}
	if !strings.Contains(template, "{configId}") && !strings.Contains(template, "{version}") {
		u, err := parseNotifyURL(template)
		if err != nil || u.Path != NotifyPath {
			return "", errNotifyURLInvalid
		}
		u.Path = fmt.Sprintf("%s/%d/%d", NotifyPath, id, version)
		if len(u.String()) > 2048 {
			return "", errNotifyURLInvalid
		}
		return u.String(), nil
	}
	raw := strings.ReplaceAll(strings.ReplaceAll(template, "{configId}", strconv.FormatInt(id, 10)), "{version}", strconv.FormatInt(version, 10))
	u, err := parseNotifyURL(raw)
	if err != nil || !strings.HasSuffix(template, NotifyPathTemplate) || u.Path != fmt.Sprintf("%s/%d/%d", NotifyPath, id, version) {
		return "", errNotifyURLInvalid
	}
	return u.String(), nil
}
func notifyTemplate(raw string, id, version int64) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Path != fmt.Sprintf("/api/admin/v1/payment/callback/alipay/%d/%d", id, version) {
		return "", fmt.Errorf("stored callback coordinates invalid")
	}
	template := u.Scheme + "://" + u.Host + NotifyPathTemplate
	resolved, err := resolveNotifyURL(template, id, version)
	if err != nil || resolved != raw {
		return "", fmt.Errorf("stored callback URL invalid")
	}
	return template, nil
}

func validateVersionNotifyURL(raw string) error {
	if strings.Contains(raw, "{configId}") || strings.Contains(raw, "{version}") {
		_, err := resolveNotifyURL(raw, 1, 1)
		return err
	}
	u, err := parseNotifyURL(raw)
	if err != nil {
		return errNotifyURLInvalid
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, NotifyPath+"/"), "/")
	if len(parts) != 2 || !strings.HasPrefix(u.Path, NotifyPath+"/") {
		return errNotifyURLInvalid
	}
	for _, part := range parts {
		value, parseErr := strconv.ParseInt(part, 10, 64)
		if parseErr != nil || value < 1 || strconv.FormatInt(value, 10) != part {
			return errNotifyURLInvalid
		}
	}
	return nil
}

func parseNotifyURL(raw string) (*url.URL, error) {
	if len(raw) > 2048 || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "?#") {
		return nil, errNotifyURLInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || strings.EqualFold(u.Hostname(), "HOST") || u.User != nil || u.RawPath != "" || u.Opaque != "" || strings.HasSuffix(u.Host, ":") || strings.ContainsAny(u.Host, "\\/\t\r\n @*") {
		return nil, errNotifyURLInvalid
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, errNotifyURLInvalid
		}
	}
	return u, nil
}

type createRequest struct {
	Name          *string              `json:"name"`
	Provider      *payment.Provider    `json:"provider"`
	Environment   *payment.Environment `json:"environment"`
	AppID         *string              `json:"appId"`
	SellerID      *string              `json:"sellerId"`
	PrivateKey    *string              `json:"privateKey"`
	AppCertPEM    *string              `json:"appCertPEM"`
	AlipayCertPEM *string              `json:"alipayCertPEM"`
	RootCertPEM   *string              `json:"rootCertPEM"`
	NotifyURL     *string              `json:"notifyUrl"`
	PayMethods    []payment.PayMethod  `json:"payMethods"`
	SortOrder     *int32               `json:"sortOrder"`
	Status        *yesno.Value         `json:"status"`
}

func (r createRequest) input() (CreateInput, error) {
	if r.Name == nil || r.Provider == nil || r.Environment == nil || r.AppID == nil || r.SellerID == nil || r.PrivateKey == nil || r.AppCertPEM == nil || r.AlipayCertPEM == nil || r.RootCertPEM == nil || r.NotifyURL == nil || r.SortOrder == nil || r.Status == nil {
		return CreateInput{}, fmt.Errorf("every creation field required")
	}
	v := CreateInput{*r.Name, *r.AppID, *r.SellerID, *r.Provider, *r.Environment, *r.PrivateKey, *r.AppCertPEM, *r.AlipayCertPEM, *r.RootCertPEM, *r.NotifyURL, r.PayMethods, *r.SortOrder, *r.Status}
	err := validateCreate(v)
	if err != nil {
		return CreateInput{}, err
	}
	v.PayMethods, err = payment.NormalizePayMethods(v.PayMethods)
	return v, err
}

type metadataRequest struct {
	Name      *string `json:"name"`
	SortOrder *int32  `json:"sortOrder"`
}

func (r metadataRequest) input() (MetadataInput, error) {
	if r.Name == nil || r.SortOrder == nil {
		return MetadataInput{}, fmt.Errorf("metadata fields required")
	}
	v := MetadataInput{*r.Name, *r.SortOrder}
	return v, validateMetadata(v)
}

type versionRequest struct {
	PrivateKey    json.RawMessage     `json:"privateKey"`
	AppCertPEM    json.RawMessage     `json:"appCertPEM"`
	AlipayCertPEM json.RawMessage     `json:"alipayCertPEM"`
	RootCertPEM   json.RawMessage     `json:"rootCertPEM"`
	NotifyURL     *string             `json:"notifyUrl"`
	PayMethods    []payment.PayMethod `json:"payMethods"`
}

func replacement(raw json.RawMessage) (*string, error) {
	if raw == nil {
		return nil, fmt.Errorf("explicit credential replacement or null required")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("credential replacement invalid")
	}
	return &s, nil
}
func (r versionRequest) input() (VersionInput, error) {
	var v VersionInput
	var err error
	v.PrivateKey, err = replacement(r.PrivateKey)
	if err != nil {
		return v, err
	}
	v.AppCertPEM, err = replacement(r.AppCertPEM)
	if err != nil {
		return v, err
	}
	v.AlipayCertPEM, err = replacement(r.AlipayCertPEM)
	if err != nil {
		return v, err
	}
	v.RootCertPEM, err = replacement(r.RootCertPEM)
	if err != nil {
		return v, err
	}
	if r.NotifyURL == nil {
		return v, fmt.Errorf("notifyUrl required")
	}
	v.NotifyURL = *r.NotifyURL
	if err = validateVersionNotifyURL(v.NotifyURL); err != nil {
		return v, err
	}
	v.PayMethods, err = payment.NormalizePayMethods(r.PayMethods)
	return v, err
}

type statusRequest struct {
	Status *yesno.Value `json:"status"`
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "status") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	p, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	q := ListQuery{Page: p.Page, PageSize: p.PageSize, Keyword: strings.TrimSpace(values.Get("keyword"))}
	if utf8.RuneCountInString(q.Keyword) > 128 || (q.Page-1) > (int(^uint(0)>>1)/q.PageSize) {
		return q, fmt.Errorf("query bounds invalid")
	}
	if vs, exists := values["status"]; exists {
		switch vs[0] {
		case "0":
			v := yesno.No
			q.Status = &v
		case "1":
			v := yesno.Yes
			q.Status = &v
		default:
			return q, fmt.Errorf("status invalid")
		}
	}
	return q, nil
}
