package uploadrule

import (
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

type listResponse struct {
	List     []RuleValue `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

func pageResponse(r pagination.Result[RuleValue]) listResponse {
	return listResponse{r.List, r.Total, r.Page, r.PageSize}
}

type PlatformOption struct {
	ID        int64       `json:"id"`
	Code      string      `json:"code"`
	Name      string      `json:"name"`
	IsEnabled yesno.Value `json:"isEnabled"`
}
type ConfigSummary struct {
	ID        int64       `json:"id"`
	Name      string      `json:"name"`
	Bucket    string      `json:"bucket"`
	Region    string      `json:"region"`
	IsEnabled yesno.Value `json:"isEnabled"`
}
type PageInit struct {
	Platforms []PlatformOption `json:"platforms"`
	Configs   []ConfigSummary  `json:"configs"`
}
type idResponse struct {
	ID int64 `json:"id"`
}
type statusResponse struct {
	ID        int64 `json:"id"`
	IsEnabled int16 `json:"isEnabled"`
}
type emptyResponse struct{}
