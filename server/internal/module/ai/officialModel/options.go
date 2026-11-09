package officialmodel

import "context"

// Vendor is the persisted provider enum for official models.
// Values are intentionally stable and domain-local; they are not a generic
// dictionary shared with other modules.
type Vendor int16

const (
	VendorOpenAI    Vendor = 1
	VendorAnthropic Vendor = 2
	VendorGoogle    Vendor = 3
)

type VendorOption struct {
	Value Vendor `json:"value"`
	Label string `json:"label"`
}

type Options struct {
	Vendors []VendorOption `json:"vendors"`
}

type vendorDefinition struct {
	value Vendor
	code  string
	label string
}

var vendorDefinitions = []vendorDefinition{
	{value: VendorOpenAI, code: "openai", label: "OpenAI"},
	{value: VendorAnthropic, code: "anthropic", label: "Anthropic"},
	{value: VendorGoogle, code: "google", label: "Google"},
}

func (vendor Vendor) Valid() bool {
	for _, definition := range vendorDefinitions {
		if definition.value == vendor {
			return true
		}
	}
	return false
}

func (vendor Vendor) Code() string {
	for _, definition := range vendorDefinitions {
		if definition.value == vendor {
			return definition.code
		}
	}
	return ""
}

func (vendor Vendor) Label() string {
	for _, definition := range vendorDefinitions {
		if definition.value == vendor {
			return definition.label
		}
	}
	return ""
}

func vendorFromCode(code string) (Vendor, bool) {
	for _, definition := range vendorDefinitions {
		if definition.code == code {
			return definition.value, true
		}
	}
	return 0, false
}

func vendorOptions() []VendorOption {
	options := make([]VendorOption, 0, len(vendorDefinitions))
	for _, definition := range vendorDefinitions {
		options = append(options, VendorOption{Value: definition.value, Label: definition.label})
	}
	return options
}

// Options returns the static vendor dictionary. Brand labels are not localized.
func (s *Service) Options(context.Context) (Options, error) {
	return Options{Vendors: vendorOptions()}, nil
}
