package recipientrule

import (
	"admin/server/internal/shared/cacheGeneration"
	"context"

	"admin/server/internal/shared/yesno"
)

type Scope int16

const (
	ScopeEmail  Scope = 0
	ScopeDomain Scope = 1
)

func (s Scope) IsValid() bool { return s == ScopeEmail || s == ScopeDomain }

type Action int16

const (
	ActionDeny  Action = 0
	ActionAllow Action = 1
)

func (a Action) IsValid() bool { return a == ActionDeny || a == ActionAllow }

const (
	PermissionCreate = "message:mail:rule:create"
	PermissionUpdate = "message:mail:rule:update"
	PermissionStatus = "message:mail:rule:status"
	PermissionDelete = "message:mail:rule:delete"
	PermissionImport = "message:mail:rule:import"
	PermissionExport = "message:mail:rule:export"
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
	Mutate(context.Context, func(context.Context, int64) (cachegeneration.MutationResult, error)) error
}

type Input struct {
	Scope     Scope       `json:"scope"`
	Pattern   string      `json:"pattern"`
	Action    Action      `json:"action"`
	Name      string      `json:"name"`
	Remark    string      `json:"remark"`
	IsEnabled yesno.Value `json:"isEnabled"`
}

const (
	XlsxMaxRows  = 1000
	XlsxMaxBytes = 2 << 20
)

type XlsxRow struct {
	Line      int      `json:"line"`
	RawValues []string `json:"rawValues"`
	Data      *Input   `json:"data"`
	Errors    []string `json:"errors"`
}

type XlsxPreview struct {
	Rows   []XlsxRow `json:"rows"`
	Errors []string  `json:"errors"`
}

type XlsxImportResult struct {
	Imported int `json:"imported"`
}
type XlsxExportFile struct {
	FileName      string `json:"fileName"`
	ContentBase64 string `json:"contentBase64"`
}
type ImportTemplate struct {
	ObjectKey string `json:"objectKey"`
}

type XlsxImportInput struct {
	FileName      string `json:"fileName"`
	ContentBase64 string `json:"contentBase64"`
}
