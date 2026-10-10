package rateLimitPolicy

type InputConstraints struct {
	MinLimit         int `json:"minLimit"`
	MaxLimit         int `json:"maxLimit"`
	MinWindowSeconds int `json:"minWindowSeconds"`
	MaxWindowSeconds int `json:"maxWindowSeconds"`
}

func Constraints() InputConstraints {
	return InputConstraints{minLimit, maxLimit, minWindowSeconds, maxWindowSeconds}
}
