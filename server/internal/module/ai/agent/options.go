package agent

import (
	"admin/server/internal/shared/option"
	"context"
)

func Options(ctx context.Context) any {
	return struct {
		Statuses      []option.Option[int16]  `json:"statuses"`
		Capabilities  []option.Option[string] `json:"capabilities"`
		TextProtocols []option.Option[int16]  `json:"textProtocols"`
		Defaults      map[string]any          `json:"defaults"`
		Constraints   map[string]int          `json:"constraints"`
	}{Statuses: []option.Option[int16]{option.New(ctx, int16(0), "停用", "Disabled"), option.New(ctx, int16(1), "启用", "Enabled")}, Capabilities: []option.Option[string]{option.New(ctx, "text", "文字", "Text"), option.New(ctx, "image", "图片", "Image"), option.New(ctx, "video", "视频", "Video"), option.New(ctx, "audio", "音频", "Audio")}, TextProtocols: []option.Option[int16]{option.New(ctx, int16(1), "Responses", "Responses"), option.New(ctx, int16(2), "Chat Completions", "Chat Completions")}, Defaults: map[string]any{"status": 0, "multiplierPpm": 1000000, "textProtocol": 1}, Constraints: map[string]int{"maxPageSize": 100, "maxPromptBytes": 65536, "maxBoundTools": 16, "maxToolCalls": 16, "minMultiplierPpm": 1, "maxMultiplierPpm": 100000000}}
}
