package template

import "context"

const (
	PermissionUpdate = "message:mail:template:update"
	PermissionStatus = "message:mail:template:status"
)

type UpdateInput struct {
	Scene             string            `json:"scene"`
	Name              string            `json:"name"`
	Subject           string            `json:"subject"`
	TencentTemplateID int               `json:"tencentTemplateId"`
	Variables         map[string]string `json:"variables"`
	ExampleVariables  map[string]string `json:"exampleVariables"`
}

type ReadinessCoordinator interface {
	Mutate(context.Context, func(context.Context) error) error
}
