package yesno

type Value int16

const (
	No  Value = 0
	Yes Value = 1
)

func IsValid(value Value) bool {
	return value == No || value == Yes
}
