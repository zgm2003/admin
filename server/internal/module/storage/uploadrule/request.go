package uploadrule

import (
	"admin/server/internal/shared/yesno"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

type ListQuery struct {
	Page, PageSize          int
	PlatformID, CosConfigID *int64
	Keyword                 string
	IsEnabled               *yesno.Value
}
type CreateInput struct {
	PlatformID                          int64
	Code, Name                          string
	CosConfigID                         int64
	MaxFileSizeBytes                    int64
	AllowedExtensions, AllowedMimeTypes []string
	AccessMode                          string
	IsEnabled                           yesno.Value
	Remark                              string
}
type UpdateInput struct {
	Name                                string
	CosConfigID                         int64
	MaxFileSizeBytes                    int64
	AllowedExtensions, AllowedMimeTypes []string
	AccessMode, Remark                  string
}
type FileInput struct {
	FileName      string `json:"fileName"`
	ContentType   string `json:"contentType"`
	FileSizeBytes int64  `json:"fileSizeBytes"`
}
type CredentialInput struct {
	RuleCode string      `json:"ruleCode"`
	Files    []FileInput `json:"files"`
}
type createRequest struct {
	PlatformID        int64       `json:"platformId"`
	Code              string      `json:"code"`
	Name              string      `json:"name"`
	CosConfigID       int64       `json:"cosConfigId"`
	MaxFileSizeBytes  int64       `json:"maxFileSizeBytes"`
	AllowedExtensions []string    `json:"allowedExtensions"`
	AllowedMimeTypes  []string    `json:"allowedMimeTypes"`
	AccessMode        string      `json:"accessMode"`
	IsEnabled         yesno.Value `json:"isEnabled"`
	Remark            string      `json:"remark"`
}
type updateRequest struct {
	Name              string   `json:"name"`
	CosConfigID       int64    `json:"cosConfigId"`
	MaxFileSizeBytes  int64    `json:"maxFileSizeBytes"`
	AllowedExtensions []string `json:"allowedExtensions"`
	AllowedMimeTypes  []string `json:"allowedMimeTypes"`
	AccessMode        string   `json:"accessMode"`
	Remark            string   `json:"remark"`
}
type statusRequest struct {
	IsEnabled *yesno.Value `json:"isEnabled"`
}

func normalize(input []string, ext bool) []string {
	out := make([]string, 0, len(input))
	seen := map[string]bool{}
	for _, v := range input {
		v = strings.ToLower(strings.TrimSpace(v))
		if ext {
			v = strings.TrimPrefix(v, ".")
		}
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func validateFields(platformID int64, code, name string, configID int64, size int64, ext, mime []string, mode, remark string, requirePlatform bool) error {
	if requirePlatform && platformID < 1 || configID < 1 {
		return fmt.Errorf("platformId/cosConfigId invalid")
	}
	if strings.TrimSpace(code) == "" || len(code) > 64 || strings.HasPrefix(code, "/") || strings.HasSuffix(code, "/") || strings.Contains(code, "..") || strings.IndexFunc(code, func(r rune) bool { return unicode.IsControl(r) }) >= 0 || strings.TrimSpace(name) == "" || len(name) > 128 {
		return fmt.Errorf("code/name invalid")
	}
	if size < 1 || size > 5*1024*1024*1024 {
		return fmt.Errorf("file limits invalid")
	}
	if len(ext) == 0 {
		return fmt.Errorf("allowedExtensions required")
	}
	if mode != "private" && mode != "public" {
		return fmt.Errorf("accessMode invalid")
	}
	if len(remark) > 512 {
		return fmt.Errorf("remark invalid")
	}
	return nil
}
func (r createRequest) input() (CreateInput, error) {
	r.Code = strings.TrimSpace(r.Code)
	r.Name = strings.TrimSpace(r.Name)
	r.Remark = strings.TrimSpace(r.Remark)
	r.AllowedExtensions = normalize(r.AllowedExtensions, true)
	r.AllowedMimeTypes = normalize(r.AllowedMimeTypes, false)
	if err := validateFields(r.PlatformID, r.Code, r.Name, r.CosConfigID, r.MaxFileSizeBytes, r.AllowedExtensions, r.AllowedMimeTypes, r.AccessMode, r.Remark, true); err != nil {
		return CreateInput{}, err
	}
	if !yesno.IsValid(r.IsEnabled) {
		return CreateInput{}, fmt.Errorf("isEnabled invalid")
	}
	return CreateInput{r.PlatformID, r.Code, r.Name, r.CosConfigID, r.MaxFileSizeBytes, r.AllowedExtensions, r.AllowedMimeTypes, r.AccessMode, r.IsEnabled, r.Remark}, nil
}
func (r updateRequest) input() (UpdateInput, error) {
	r.Name = strings.TrimSpace(r.Name)
	r.Remark = strings.TrimSpace(r.Remark)
	r.AllowedExtensions = normalize(r.AllowedExtensions, true)
	r.AllowedMimeTypes = normalize(r.AllowedMimeTypes, false)
	if err := validateFields(1, "x", r.Name, r.CosConfigID, r.MaxFileSizeBytes, r.AllowedExtensions, r.AllowedMimeTypes, r.AccessMode, r.Remark, false); err != nil {
		return UpdateInput{}, err
	}
	return UpdateInput{r.Name, r.CosConfigID, r.MaxFileSizeBytes, r.AllowedExtensions, r.AllowedMimeTypes, r.AccessMode, r.Remark}, nil
}
func parseListQuery(v url.Values) (ListQuery, error) {
	allowed := map[string]bool{"page": true, "pageSize": true, "platformId": true, "cosConfigId": true, "keyword": true, "isEnabled": true}
	for key, values := range v {
		if !allowed[key] || len(values) != 1 {
			return ListQuery{}, fmt.Errorf("invalid list query")
		}
	}
	q := ListQuery{}
	var err error
	q.Page, err = strconv.Atoi(v.Get("page"))
	if err != nil || q.Page < 1 {
		return q, fmt.Errorf("page invalid")
	}
	q.PageSize, err = strconv.Atoi(v.Get("pageSize"))
	if err != nil || q.PageSize < 1 || q.PageSize > 100 {
		return q, fmt.Errorf("pageSize invalid")
	}
	q.Keyword = strings.TrimSpace(v.Get("keyword"))
	for key, target := range map[string]**int64{"platformId": &q.PlatformID, "cosConfigId": &q.CosConfigID} {
		if raw := v.Get(key); raw != "" {
			id, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || id < 1 {
				return q, fmt.Errorf("%s invalid", key)
			}
			*target = &id
		}
	}
	if raw := v.Get("isEnabled"); raw != "" {
		n, e := strconv.Atoi(raw)
		val := yesno.Value(n)
		if e != nil || !yesno.IsValid(val) {
			return q, fmt.Errorf("isEnabled invalid")
		}
		q.IsEnabled = &val
	}
	return q, nil
}

var _ = json.RawMessage{}
