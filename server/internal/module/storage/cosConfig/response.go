package cosconfig

import "admin/server/internal/shared/pagination"

type listResponse struct {
	List     []SafeValue `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

func safeValue(row Current) SafeValue {
	return SafeValue{ID: row.ID, Name: row.Name, AppID: row.AppID, Bucket: row.Bucket, Region: row.Region, Endpoint: copiedStringPointer(row.Endpoint), BucketDomain: copiedStringPointer(row.BucketDomain), IsEnabled: row.IsEnabled, HasCredentials: row.SecretIDCiphertext != "" && row.SecretKeyCiphertext != "", Remark: row.Remark, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
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
