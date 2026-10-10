package role

const roleNameMinLength = 1
const roleNameMaxLength = 64

type FormOptions struct {
	CodePattern   string `json:"codePattern"`
	NameMinLength int    `json:"nameMinLength"`
	NameMaxLength int    `json:"nameMaxLength"`
}

func RoleFormOptions() FormOptions {
	return FormOptions{CodePattern: roleCodePattern.String(), NameMinLength: roleNameMinLength, NameMaxLength: roleNameMaxLength}
}
