package ratelimitpolicy

import "testing"

func TestConstraintsDescribeValidatorBoundaries(t *testing.T) {
	c := Constraints()
	for _, input := range []RateLimitPolicyInput{
		{Key: FixedRateLimitPolicies()[0].Key, Limit: c.MinLimit, WindowSeconds: c.MinWindowSeconds},
		{Key: FixedRateLimitPolicies()[0].Key, Limit: c.MaxLimit, WindowSeconds: c.MaxWindowSeconds},
	} {
		if err := ValidateInput(input); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateInput(RateLimitPolicyInput{Key: FixedRateLimitPolicies()[0].Key, Limit: c.MaxLimit + 1, WindowSeconds: c.MinWindowSeconds}); err == nil {
		t.Fatal("accepted above maximum")
	}
}
