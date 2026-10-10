package ratelimitpolicy

const (
	minLimit         = 1
	maxLimit         = 100000
	minWindowSeconds = 1
	maxWindowSeconds = 86400
)

type InputConstraints struct {
	MinLimit         int `json:"minLimit"`
	MaxLimit         int `json:"maxLimit"`
	MinWindowSeconds int `json:"minWindowSeconds"`
	MaxWindowSeconds int `json:"maxWindowSeconds"`
}

func Constraints() InputConstraints {
	return InputConstraints{minLimit, maxLimit, minWindowSeconds, maxWindowSeconds}
}
