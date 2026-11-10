package officialmodel

import (
	"context"
	"testing"
)

func TestVendorOptions(t *testing.T) {
	options, err := (&Service{}).Options(context.Background())
	if err != nil {
		t.Fatalf("Options() error = %v", err)
	}
	if len(options.Vendors) != 4 {
		t.Fatalf("vendors = %+v", options.Vendors)
	}
	want := []VendorOption{
		{Value: VendorOpenAI, Label: "OpenAI"},
		{Value: VendorAnthropic, Label: "Anthropic"},
		{Value: VendorGoogle, Label: "Google"},
		{Value: VendorVolcengine, Label: "火山引擎"},
	}
	for index, option := range options.Vendors {
		if option != want[index] {
			t.Fatalf("vendors[%d] = %+v", index, option)
		}
	}
}
