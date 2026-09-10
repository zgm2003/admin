package dictionary

import "time"

type emptyResponse struct{}
type idResponse struct {
	ID int64 `json:"id"`
}
type statusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}
type listRow struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	NameZH      string `json:"nameZh"`
	NameEN      string `json:"nameEn"`
	Description string `json:"description"`
	IsEnabled   int16  `json:"isEnabled"`
	IsBuiltin   int16  `json:"isBuiltin"`
	ItemCount   int64  `json:"itemCount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
type listResponse struct {
	List     []listRow `json:"list"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

type itemResponse struct {
	ID           int64  `json:"id"`
	DictionaryID int64  `json:"dictionaryId"`
	Value        string `json:"value"`
	LabelZH      string `json:"labelZh"`
	LabelEN      string `json:"labelEn"`
	Sort         int    `json:"sort"`
	IsEnabled    int16  `json:"isEnabled"`
	IsBuiltin    int16  `json:"isBuiltin"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
type detailResponse struct {
	Dictionary listRow        `json:"dictionary"`
	Items      []itemResponse `json:"items"`
}

func dictionaryListResponse(result ListResult) listResponse {
	rows := make([]listRow, 0, len(result.List))
	for _, item := range result.List {
		rows = append(rows, listRow{ID: item.ID, Code: item.Code, NameZH: item.NameZH, NameEN: item.NameEN, Description: item.Description, IsEnabled: int16(item.IsEnabled), IsBuiltin: int16(item.IsBuiltin), ItemCount: item.ItemCount, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return listResponse{List: rows, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

func dictionaryDetailResponse(value Detail) detailResponse {
	row := dictionaryListResponse(ListResult{List: []ListItem{{Dictionary: value.Dictionary, ItemCount: int64(len(value.Items))}}}).List[0]
	items := make([]itemResponse, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, itemResponse{ID: item.ID, DictionaryID: item.DictionaryID, Value: item.Value, LabelZH: item.LabelZH, LabelEN: item.LabelEN, Sort: item.Sort, IsEnabled: int16(item.IsEnabled), IsBuiltin: int16(item.IsBuiltin), CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return detailResponse{Dictionary: row, Items: items}
}
