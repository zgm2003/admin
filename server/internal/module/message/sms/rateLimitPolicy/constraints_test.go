package rateLimitPolicy

import "testing"

func TestConstraintsDescribeValidatorBoundaries(t *testing.T) {
	c := Constraints()
	if err := ValidateInput(KeyMinute, c.MinLimit, c.MinWindowSeconds); err != nil {
		t.Fatal(err)
	}
	if err := ValidateInput(KeyMinute, c.MaxLimit, c.MaxWindowSeconds); err != nil {
		t.Fatal(err)
	}
	if err := ValidateInput(KeyMinute, c.MaxLimit+1, c.MinWindowSeconds); err == nil {
		t.Fatal("accepted above maximum")
	}
}
