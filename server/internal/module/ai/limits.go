package ai

// Engineering limits are fixed at build time. They are intentionally not
// admin-editable system settings: model capabilities belong to
// ai_provider_model.limits and deployment capacity belongs to environment
// configuration.
const (
	MaxModelRounds      int64 = 8
	MaxToolCalls        int64 = 16
	MaxBoundTools       int64 = 16
	MaxInputAttachments int64 = 4
	MaxTextInputBytes   int64 = 128 * 1024

	TextTimeoutSeconds  int64 = 180
	ImageTimeoutSeconds int64 = 300
	AudioTimeoutSeconds int64 = 300
	VideoTimeoutSeconds int64 = 3600

	DownloadTimeoutSeconds int64 = 300
	TransferTimeoutSeconds int64 = 900

	ImageMaxBytes int64 = 10 * 1024 * 1024
	AudioMaxBytes int64 = 100 * 1024 * 1024
	VideoMaxBytes int64 = 500 * 1024 * 1024
)
