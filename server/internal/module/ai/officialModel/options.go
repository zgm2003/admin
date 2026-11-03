package officialmodel

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
	ops := []stringOption{{"text.generate", "Text generation"}, {"image.generate", "Image generation"}, {"image.edit", "Image editing"}, {"audio.speech", "Speech generation"}, {"audio.transcribe", "Audio transcription"}}
	if zh {
		disabled, enabled = "停用", "启用"
		labels := []string{"文字生成", "图片生成", "图片编辑", "语音生成", "音频转写"}
		for i := range ops {
			ops[i].Label = labels[i]
		}
	}
	response.OK(c, 200, map[string]any{"statuses": []numericOption{{0, disabled}, {1, enabled}}, "operations": ops, "defaults": map[string]int{"status": 0}, "constraints": map[string]int{"maxNameLength": 100, "maxModelKeyLength": 200, "maxSourceUrls": 32}})
}
