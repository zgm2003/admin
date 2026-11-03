package provider

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type numericOption struct {
	Value int16  `json:"value"`
	Label string `json:"label"`
}
type stringOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func (h *Handler) Options(c *gin.Context) {
	zh := i18n.LocaleFromContext(c.Request.Context()) == i18n.ZhCN
	disabled, enabled := "Disabled", "Enabled"
	if zh {
		disabled, enabled = "停用", "启用"
	}
	response.OK(c, 200, map[string]any{"statuses": []numericOption{{0, disabled}, {1, enabled}}, "textProtocols": []numericOption{{1, "Responses"}, {2, "Chat Completions"}}, "operations": []stringOption{{"text.generate", label(zh, "文字生成", "Text generation")}, {"image.generate", label(zh, "图片生成", "Image generation")}, {"image.edit", label(zh, "图片编辑", "Image editing")}, {"audio.speech", label(zh, "语音生成", "Speech generation")}, {"audio.transcribe", label(zh, "音频转写", "Audio transcription")}}, "defaults": map[string]any{"requestTimeoutMs": 30000, "status": 0}, "constraints": map[string]any{"requestTimeoutMs": map[string]int{"min": 1000, "max": 300000}, "maxNameLength": 100, "maxDescriptionLength": 1000}})
}
func label(zh bool, a, b string) string {
	if zh {
		return a
	}
	return b
}
