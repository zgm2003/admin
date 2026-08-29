package cosconfig

import "admin/server/internal/shared/pagination"

type listResponse struct {
	List     []SafeValue `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

func safeValue(m Model) SafeValue {
	return SafeValue{ID: m.ID, Name: m.Name, AppID: m.AppID, Bucket: m.Bucket, Region: m.Region, Endpoint: copiedStringPointer(m.Endpoint), BucketDomain: copiedStringPointer(m.BucketDomain), IsEnabled: m.IsEnabled, HasCredentials: m.SecretIDCiphertext != "" && m.SecretKeyCiphertext != "", Remark: m.Remark, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func copiedStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func pageResponse(result pagination.Result[SafeValue]) listResponse {
	return listResponse{List: result.List, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

type idResponse struct {
	ID int64 `json:"id"`
}
type statusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}
type emptyResponse struct{}
