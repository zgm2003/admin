package provider

type Vendor int16

const (
	VendorOpenAI     Vendor = 1
	VendorAnthropic  Vendor = 2
	VendorGoogle     Vendor = 3
	VendorVolcengine Vendor = 4
)

func (vendor Vendor) Valid() bool {
	return vendor >= VendorOpenAI && vendor <= VendorVolcengine
}

func (vendor Vendor) Label() string {
	switch vendor {
	case VendorOpenAI:
		return "OpenAI"
	case VendorAnthropic:
		return "Anthropic"
	case VendorGoogle:
		return "Google"
	case VendorVolcengine:
		return "火山引擎"
	default:
		return ""
	}
}

type Protocol int16

const (
	ProtocolOpenAI          Protocol = 1
	ProtocolAnthropic       Protocol = 2
	ProtocolGemini          Protocol = 3
	ProtocolOpenAIResponses Protocol = 4
)

func (protocol Protocol) Valid() bool {
	return protocol >= ProtocolOpenAI && protocol <= ProtocolOpenAIResponses
}
