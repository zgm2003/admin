package run

import "testing"

func TestPricingMicrosToWalletUnits(t *testing.T) {
	value, err := pricingMicrosToWalletUnits(7294)
	if err != nil {
		t.Fatal(err)
	}
	if value != 729400 {
		t.Fatalf("units = %d", value)
	}
	if _, err := pricingMicrosToWalletUnits(-1); err == nil {
		t.Fatal("negative pricing amount was accepted")
	}
}

func TestTitleFromContentNormalizesWhitespace(t *testing.T) {
	title := titleFromContent("  line one\nline two  ")
	if title != "line one line two" {
		t.Fatalf("title = %q", title)
	}
}
