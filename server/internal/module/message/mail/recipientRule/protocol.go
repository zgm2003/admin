package recipientrule

import (
	"context"

	"admin/server/internal/shared/yesno"
)

const (
	ScopeEmail  = "email"
	ScopeDomain = "domain"
	ActionAllow = "allow"
	ActionDeny  = "deny"

	PermissionCreate = "message:mail:rule:create"
	PermissionUpdate = "message:mail:rule:update"
	PermissionStatus = "message:mail:rule:status"
	PermissionDelete = "message:mail:rule:delete"
)

type SendMode string

const (
	SendModeBusiness  SendMode = "business"
	SendModeAdminTest SendMode = "admin_test"
)

type Decision struct {
	Allowed bool
	RuleID  int64
	Reason  string
}

type Evaluator interface {
	Evaluate(context.Context, string, SendMode) (Decision, error)
}

type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}

type Input struct {
	Scope     string      `json:"scope"`
	Pattern   string      `json:"pattern"`
	Action    string      `json:"action"`
	Name      string      `json:"name"`
	Remark    string      `json:"remark"`
	IsEnabled yesno.Value `json:"isEnabled"`
}
