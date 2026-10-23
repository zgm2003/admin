package account

import "time"

type emptyResponse struct{}

type roleSummaryResponse struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Status int16  `json:"status"`
}

type listItemResponse struct {
	Actions      Actions               `json:"actions"`
	ActionLabels ActionLabels          `json:"actionLabels"`
	ID           int64                 `json:"id"`
	Username     string                `json:"username"`
	Email        string                `json:"email"`
	Phone        *string               `json:"phone"`
	Status    int16                 `json:"status"`
	Roles        []roleSummaryResponse `json:"roles"`
	CreatedAt    string                `json:"createdAt"`
	UpdatedAt    string                `json:"updatedAt"`
}

type listResponse struct {
	List     []listItemResponse `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type roleOptionsResponse struct {
	Roles []roleSummaryResponse `json:"roles"`
}
type updatedProfileResponse struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	Phone     *string `json:"phone"`
	UpdatedAt string  `json:"updatedAt"`
}
type statusResponse struct {
	ID        int64 `json:"id"`
	Status int16 `json:"status"`
}
type summaryResponse struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone"`
	Status int16   `json:"status"`
}
type rolesResponse struct {
	User    summaryResponse          `json:"user"`
	Roles   []assignmentRoleResponse `json:"roles"`
	RoleIDs []int64                  `json:"roleIds"`
}
type roleResultResponse struct {
	ID        int64 `json:"id"`
	RoleCount int64 `json:"roleCount"`
}

func userListResponse(items []ListItem, total int64, page, pageSize int) listResponse {
	rows := make([]listItemResponse, 0, len(items))
	for _, item := range items {
		rows = append(rows, listItemResponse{Actions: item.Actions, ActionLabels: item.ActionLabels, ID: item.ID, Username: item.Username, Email: item.Email, Phone: item.Phone, Status: int16(item.Status), Roles: roleSummaryResponses(item.Roles), CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return listResponse{List: rows, Total: total, Page: page, PageSize: pageSize}
}

func roleSummaryResponses(values []RoleSummary) []roleSummaryResponse {
	result := make([]roleSummaryResponse, 0, len(values))
	for _, value := range values {
		result = append(result, roleSummaryResponse{ID: value.ID, Code: value.Code, Name: value.Name, Status: int16(value.Status)})
	}
	return result
}

func newRolesResponse(value Roles) rolesResponse {
	return rolesResponse{User: summaryResponse{ID: value.User.ID, Username: value.User.Username, Email: value.User.Email, Phone: value.User.Phone, Status: int16(value.User.Status)}, Roles: assignmentRoleResponses(value.Roles), RoleIDs: append([]int64{}, value.RoleIDs...)}
}

type assignmentRoleResponse struct {
	roleSummaryResponse
	Selectable bool `json:"selectable"`
	Locked     bool `json:"locked"`
}

func assignmentRoleResponses(values []AssignmentRole) []assignmentRoleResponse {
	result := make([]assignmentRoleResponse, 0, len(values))
	for _, value := range values {
		result = append(result, assignmentRoleResponse{roleSummaryResponse: roleSummaryResponse{ID: value.ID, Code: value.Code, Name: value.Name, Status: int16(value.Status)}, Selectable: value.Selectable, Locked: value.Locked})
	}
	return result
}
