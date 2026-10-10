package account

const (
	usernameMinLength = 3
	usernameMaxLength = 64
	usernamePattern   = `^[\p{L}\p{Nd}_-]+$`
)

type FormOptions struct {
	UsernameMinLength int    `json:"usernameMinLength"`
	UsernameMaxLength int    `json:"usernameMaxLength"`
	UsernamePattern   string `json:"usernamePattern"`
}

func AccountFormOptions() FormOptions {
	return FormOptions{UsernameMinLength: usernameMinLength, UsernameMaxLength: usernameMaxLength, UsernamePattern: usernamePattern}
}
