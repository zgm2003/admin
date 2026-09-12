package setting

type listResponse struct {
	List     []listItem `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}
type detailResponse struct {
	Setting listItem `json:"setting"`
}
type idResponse struct {
	ID int64 `json:"id"`
}
type statusResponse struct {
	Key       string `json:"key"`
	IsEnabled int16  `json:"isEnabled"`
}

func itemResponse(row Record) listItem {
	return listItem{ID: row.ID, Key: row.Key, Value: row.Value, ValueType: row.ValueType, Description: row.Description, IsEnabled: row.IsEnabled, IsBuiltin: row.IsBuiltin, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}
func listResultResponse(result ListResult) listResponse {
	items := make([]listItem, 0, len(result.Items))
	for _, row := range result.Items {
		items = append(items, itemResponse(row))
	}
	return listResponse{List: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}
