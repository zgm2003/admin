package run

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"admin/server/internal/module/ai/chat"
	auth "admin/server/internal/module/auth/login"
)

const (
	defaultContextWindowTokens = 128000
	defaultMaxOutputTokens     = 8192
	contextSafetyTokens        = 1024
	minContextInputTokens      = 512
	imageContextTokens         = 1000
	messageContextOverhead     = 4
)

type contextPlan struct {
	WindowTokens         int  `json:"windowTokens"`
	MaxOutputTokens      int  `json:"maxOutputTokens"`
	InputBudgetTokens    int  `json:"inputBudgetTokens"`
	EstimatedTokens      int  `json:"estimatedTokens"`
	SystemTokens         int  `json:"systemTokens"`
	ToolTokens           int  `json:"toolTokens"`
	IncludedMessages     int  `json:"includedMessages"`
	DroppedMessages      int  `json:"droppedMessages"`
	TruncatedAttachments int  `json:"truncatedAttachments"`
	OverBudget           bool `json:"overBudget"`
}

type contextContentPart struct {
	name   string
	text   string
	isText bool
}

type contextMessageContent struct {
	base        string
	parts       []contextContentPart
	attachments []ChatAttachment
}

func resolveContextLimits(agent AgentConfig) (windowTokens, maxOutputTokens int) {
	windowTokens = defaultContextWindowTokens
	maxOutputTokens = defaultMaxOutputTokens
	if agent.ContextWindowTokens != nil && *agent.ContextWindowTokens >= 1024 {
		windowTokens = int(*agent.ContextWindowTokens)
	}
	if agent.MaxOutputTokens != nil && *agent.MaxOutputTokens > 0 {
		maxOutputTokens = int(*agent.MaxOutputTokens)
	}
	if maxOutputTokens >= windowTokens {
		maxOutputTokens = windowTokens / 4
	}
	if maxOutputTokens < 1 {
		maxOutputTokens = 1
	}
	return windowTokens, maxOutputTokens
}

// estimateTextTokens is deliberately conservative: CJK characters are treated
// as two tokens each and other text as roughly three characters per token.
func estimateTextTokens(value string) int {
	if value == "" {
		return 0
	}
	var cjk, other int
	for _, char := range value {
		if char >= 0x2E80 {
			cjk++
		} else {
			other++
		}
	}
	return cjk*2 + (other+2)/3
}

func estimateToolsTokens(tools []ToolDefinition) int {
	if len(tools) == 0 {
		return 0
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		return len(tools) * 80
	}
	return estimateTextTokens(string(raw)) + len(tools)*messageContextOverhead
}

func estimateMessageTokens(message ChatMessage) int {
	tokens := messageContextOverhead + estimateTextTokens(message.Content)
	for _, attachment := range message.Attachments {
		if attachment.IsImage {
			tokens += imageContextTokens
		}
	}
	if len(message.ToolCalls) > 0 {
		raw, err := json.Marshal(message.ToolCalls)
		if err == nil {
			tokens += estimateTextTokens(string(raw))
		}
	}
	return tokens
}

func renderContextContent(content contextMessageContent) string {
	if len(content.parts) == 0 {
		return content.base
	}
	var builder strings.Builder
	builder.WriteString(content.base)
	for _, part := range content.parts {
		if part.isText {
			builder.WriteString("\n\n[附件: ")
			builder.WriteString(part.name)
			builder.WriteString("]\n")
			builder.WriteString(part.text)
			continue
		}
		builder.WriteString("\n\n")
		builder.WriteString(part.text)
	}
	return builder.String()
}

func (s *Service) resolveContextMessage(
	ctx context.Context, run Run, message chat.Message,
) (contextMessageContent, error) {
	content := contextMessageContent{base: message.Content}
	if len(message.Attachments) == 0 {
		return content, nil
	}
	if s.attachmentURLs == nil {
		return contextMessageContent{}, errors.New("attachment URL resolver is not configured")
	}
	identity := auth.Identity{UserID: run.UserID, PlatformID: run.PlatformID}
	for _, attachment := range message.Attachments {
		disposition := "attachment"
		if attachment.Kind == chat.AttachmentImage {
			disposition = "inline"
		}
		resolved, err := s.attachmentURLs.ObjectURL(ctx, identity, attachment.ObjectKey, disposition)
		if err != nil {
			return contextMessageContent{}, err
		}
		if attachment.Kind == chat.AttachmentImage {
			content.attachments = append(content.attachments, ChatAttachment{
				URL: resolved.URL, IsImage: true, FileName: attachment.FileName,
			})
			continue
		}
		if isTextAttachment(attachment.MimeType, attachment.FileName) {
			if text, readErr := readAttachmentText(ctx, resolved.URL); readErr == nil && text != "" {
				content.parts = append(content.parts, contextContentPart{
					name: attachment.FileName, text: text, isText: true,
				})
				continue
			}
		}
		content.parts = append(content.parts, contextContentPart{
			text: "[附件: " + attachment.FileName + "](" + resolved.URL + ")",
		})
	}
	return content, nil
}

func (s *Service) assembleContext(
	ctx context.Context, run Run, agent AgentConfig, history []chat.Message,
	attachments map[int64][]chat.MessageAttachment, systemPrompt string, tools []ToolDefinition,
) ([]ChatMessage, contextPlan, error) {
	windowTokens, maxOutputTokens := resolveContextLimits(agent)
	plan := contextPlan{
		WindowTokens: windowTokens, MaxOutputTokens: maxOutputTokens,
	}
	plan.InputBudgetTokens = windowTokens - maxOutputTokens - contextSafetyTokens
	if plan.InputBudgetTokens < minContextInputTokens {
		plan.InputBudgetTokens = windowTokens - maxOutputTokens
	}
	if plan.InputBudgetTokens < minContextInputTokens {
		plan.InputBudgetTokens = maxInt(minContextInputTokens, windowTokens/2)
	}
	plan.SystemTokens = estimateTextTokens(systemPrompt) + messageContextOverhead
	plan.ToolTokens = estimateToolsTokens(tools)
	remaining := plan.InputBudgetTokens - plan.SystemTokens - plan.ToolTokens
	if remaining < 0 {
		remaining = 0
	}

	currentUserID := int64(0)
	for _, message := range history {
		if message.Role == chat.RoleUser && (strings.TrimSpace(message.Content) != "" || len(message.Attachments) > 0) {
			currentUserID = message.ID
			break
		}
	}

	selected := make([]ChatMessage, 0, len(history))
	for _, message := range history {
		message.Attachments = attachments[message.ID]
		if (message.Content == "" && len(message.Attachments) == 0) ||
			(message.Role == chat.RoleAssistant && message.Status == chat.MessageStreaming) {
			continue
		}
		role := "user"
		if message.Role == chat.RoleAssistant {
			role = "assistant"
		}
		baseTokens := messageContextOverhead + estimateTextTokens(message.Content)
		isCurrent := message.ID == currentUserID
		if !isCurrent && baseTokens > remaining {
			plan.DroppedMessages++
			break
		}

		content, err := s.resolveContextMessage(ctx, run, message)
		if err != nil {
			return nil, plan, err
		}
		providerMessage := ChatMessage{
			Role: role, Content: renderContextContent(content), Attachments: content.attachments,
		}
		tokens := estimateMessageTokens(providerMessage)
		if isCurrent && tokens > remaining {
			excess := tokens - remaining
			for index := len(content.parts) - 1; index >= 0 && excess > 0; index-- {
				if !content.parts[index].isText {
					continue
				}
				runes := []rune(content.parts[index].text)
				trimCount := excess * 3
				if trimCount > len(runes) {
					trimCount = len(runes)
				}
				if trimCount <= 0 {
					continue
				}
				content.parts[index].text = string(runes[:len(runes)-trimCount]) + "\n\n[附件内容已截断]"
				excess -= (trimCount + 2) / 3
				plan.TruncatedAttachments++
			}
			providerMessage.Content = renderContextContent(content)
			tokens = estimateMessageTokens(providerMessage)
			if tokens > remaining {
				plan.OverBudget = true
			}
		}
		if !isCurrent && tokens > remaining {
			plan.DroppedMessages++
			break
		}
		selected = append(selected, providerMessage)
		plan.IncludedMessages++
		remaining -= tokens
		if remaining <= 0 {
			break
		}
	}

	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	plan.EstimatedTokens = plan.InputBudgetTokens - remaining
	if plan.EstimatedTokens < 0 {
		plan.EstimatedTokens = 0
	}
	return selected, plan, nil
}

func latestUserQuery(history []chat.Message) string {
	for _, message := range history {
		if message.Role != chat.RoleUser {
			continue
		}
		if query := strings.TrimSpace(message.Content); query != "" {
			return query
		}
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
