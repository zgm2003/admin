package config

import (
	"context"
	"testing"
)

func TestOptionsExposeTTLValidatorConstraints(t *testing.T) {
	options, err := (&Service{}).Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if options.Constraints.MinTTLMinutes != minTTLMinutes || options.Constraints.MaxTTLMinutes != maxTTLMinutes {
		t.Fatalf("constraints = %+v", options.Constraints)
	}
}
