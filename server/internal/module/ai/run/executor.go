package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"admin/server/internal/module/ai/chat"
	"admin/server/internal/module/ai/pricing"
	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/realtime"
	"admin/server/internal/shared/money"

	"gorm.io/gorm"
)

const (
	streamFlushInterval    = 50 * time.Millisecond
	streamFlushBytes       = 2048
	maxHistoryMessages     = 40
	maxHistoryRunes        = 60000
	maxAttachmentTextBytes = 256 * 1024
	maxAttachmentTextRunes = 20000
	attachmentReadTimeout  = 10 * time.Second
)

func isTextAttachment(mimeType, fileName string) bool {
	normalizedMime := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if strings.HasPrefix(normalizedMime, "text/") {
		return true
	}
	switch normalizedMime {
	case "application/json", "application/xml", "application/xhtml+xml",
		"application/x-yaml", "application/yaml", "application/toml",
		"application/javascript", "application/x-sh":
		return true
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".txt", ".md", ".markdown", ".csv", ".json", ".xml", ".yaml", ".yml",
		".log", ".toml", ".ini", ".conf", ".sql", ".go", ".js", ".jsx", ".ts", ".tsx",
		".py", ".java", ".kt", ".c", ".h", ".cpp", ".hpp", ".cs", ".php", ".rb",
		".rs", ".sh", ".bash", ".zsh", ".ps1", ".html", ".css", ".scss", ".less":
		return true
	default:
		return false
	}
}

func readAttachmentText(ctx context.Context, url string) (string, error) {
	readCtx, cancel := context.WithTimeout(ctx, attachmentReadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(readCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("attachment download returned %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxAttachmentTextBytes+1))
	if err != nil {
		return "", err
	}
	truncated := len(data) > maxAttachmentTextBytes
	if truncated {
		data = data[:maxAttachmentTextBytes]
	}
	content := strings.TrimPrefix(strings.ToValidUTF8(string(data), "\uFFFD"), "\uFEFF")
	content = strings.ReplaceAll(content, "\x00", "")
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) > maxAttachmentTextRunes {
		content = string(runes[:maxAttachmentTextRunes])
		truncated = true
	}
	if content == "" {
		return "", nil
	}
	if truncated {
		content += "\n\n[附件内容已截断]"
	}
	return content, nil
}

func (s *Service) Execute(parent context.Context, runID int64) error {
	if s == nil || s.repo == nil || s.chat == nil {
		return errors.New("run service is not configured")
	}
	now := s.now().UTC()
	run, claimed, err := s.repo.ClaimRun(parent, runID, now)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	assistant, err := s.chat.MessageByRunID(parent, runID)
	if err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	agent, err := s.repo.FindAgent(parent, run.AgentID)
	if err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	apiKey, err := s.decryptAPIKey(agent)
	if err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	if err := s.appendEvent(parent, runID, "started", "运行开始", map[string]any{
		"conversationId": run.ConversationID, "assistantMessageId": assistant.ID,
	}); err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	s.publish(parent, run, realtime.EventAIChatStarted, map[string]any{
		"runId": runID, "requestId": run.RequestID, "conversationId": run.ConversationID,
		"assistantMessageId": assistant.ID, "deliverySeq": assistant.DeliverySeq,
	})

	history, err := s.chat.ListMessages(parent, run.ConversationID, assistant.Seq, maxHistoryMessages)
	if err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	messageIDs := make([]int64, 0, len(history))
	for _, message := range history {
		messageIDs = append(messageIDs, message.ID)
	}
	attachments, err := s.chat.AttachmentsByMessageIDs(parent, messageIDs)
	if err != nil {
		return s.failBeforeStream(parent, run, err)
	}
	messages := make([]ChatMessage, 0, len(history))
	usedRunes := 0
	for i := len(history) - 1; i >= 0; i-- {
		message := history[i]
		message.Attachments = attachments[message.ID]
		if (message.Content == "" && len(message.Attachments) == 0) ||
			message.Role == chat.RoleAssistant && message.Status == chat.MessageStreaming {
			continue
		}
		role := "user"
		if message.Role == chat.RoleAssistant {
			role = "assistant"
		}
		content := message.Content
		providerAttachments := make([]ChatAttachment, 0, len(message.Attachments))
		for _, attachment := range message.Attachments {
			if s.attachmentURLs == nil {
				return s.failBeforeStream(parent, run, errors.New("attachment URL resolver is not configured"))
			}
			disposition := "attachment"
			if attachment.Kind == chat.AttachmentImage {
				disposition = "inline"
			}
			resolved, resolveErr := s.attachmentURLs.ObjectURL(parent, auth.Identity{
				UserID: run.UserID, PlatformID: run.PlatformID,
			}, attachment.ObjectKey, disposition)
			if resolveErr != nil {
				return s.failBeforeStream(parent, run, resolveErr)
			}
			if attachment.Kind == chat.AttachmentImage {
				providerAttachments = append(providerAttachments, ChatAttachment{
					URL: resolved.URL, IsImage: true, FileName: attachment.FileName,
				})
				continue
			}
			if isTextAttachment(attachment.MimeType, attachment.FileName) {
				if text, textErr := readAttachmentText(parent, resolved.URL); textErr == nil && text != "" {
					content += "\n\n[附件: " + attachment.FileName + "]\n" + text
					continue
				}
			}
			content += "\n\n[附件: " + attachment.FileName + "](" + resolved.URL + ")"
		}
		usedRunes += len([]rune(content))
		if usedRunes > maxHistoryRunes {
			break
		}
		messages = append(messages, ChatMessage{
			Role: role, Content: content, Attachments: providerAttachments,
		})
	}

	timeout := time.Duration(run.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	persistCtx := context.WithoutCancel(parent)
	execCtx, cancel := context.WithTimeout(persistCtx, timeout)
	defer cancel()

	var cancelOnce sync.Once
	cancelHit := make(chan struct{})
	go s.watchCancel(execCtx, runID, cancel, &cancelOnce, cancelHit)

	chunks, err := s.provider.Stream(execCtx, ProviderRequest{
		Protocol: run.ProviderProtocol, BaseURL: agent.BaseURL, APIKey: apiKey,
		Model: agent.UpstreamModel, SystemPrompt: agent.SystemPrompt, Messages: messages, Timeout: timeout,
	})
	if err != nil {
		if isCanceled(execCtx, cancelHit) || errors.Is(err, context.Canceled) {
			return s.finish(persistCtx, run, assistant, nil, RunCanceled, chat.MessageStopped, "canceled", "用户取消生成", "", nil)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return s.finish(persistCtx, run, assistant, nil, RunTimeout, chat.MessageStopped, "timeout", "生成超时", "", nil)
		}
		return s.finish(persistCtx, run, assistant, nil, RunFailed, chat.MessageFailed, "upstream_error", err.Error(), "", nil)
	}

	var content strings.Builder
	var pending strings.Builder
	var usage *ChatUsage
	deliverySeq := assistant.DeliverySeq
	lastFlush := time.Now()
	var firstTokenAt *time.Time

	flush := func(force bool) error {
		if pending.Len() == 0 {
			return nil
		}
		if !force && pending.Len() < streamFlushBytes && time.Since(lastFlush) < streamFlushInterval {
			return nil
		}
		deliverySeq++
		delta := pending.String()
		pending.Reset()
		if err := s.chat.UpdateStreamingMessage(persistCtx, assistant.ID, content.String(), deliverySeq, s.now().UTC()); err != nil {
			return err
		}
		lastFlush = time.Now()
		s.publish(persistCtx, run, realtime.EventAIChatDelta, map[string]any{
			"runId": run.ID, "conversationId": run.ConversationID,
			"assistantMessageId": assistant.ID, "deliverySeq": deliverySeq, "delta": delta,
		})
		return nil
	}

	finishWith := func(status RunStatus, messageStatus chat.MessageStatus, code, message string) error {
		if err := flush(true); err != nil {
			return err
		}
		return s.finish(persistCtx, run, assistant, usage, status, messageStatus, code, message, content.String(), firstTokenAt)
	}

	for {
		select {
		case <-execCtx.Done():
			if isCanceled(execCtx, cancelHit) {
				return finishWith(RunCanceled, chat.MessageStopped, "canceled", "用户取消生成")
			}
			return finishWith(RunTimeout, chat.MessageStopped, "timeout", "生成超时")
		case chunk, ok := <-chunks:
			if !ok {
				return finishWith(RunFailed, chat.MessageFailed, "upstream_closed", "上游连接提前结束")
			}
			if chunk.Err != nil {
				return finishWith(RunFailed, chat.MessageFailed, "upstream_stream_error", chunk.Err.Error())
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			if chunk.Delta != "" {
				if firstTokenAt == nil {
					value := time.Now()
					firstTokenAt = &value
				}
				content.WriteString(chunk.Delta)
				pending.WriteString(chunk.Delta)
				if err := flush(false); err != nil {
					return s.finish(persistCtx, run, assistant, usage, RunFailed, chat.MessageFailed, "persist_error", err.Error(), content.String(), firstTokenAt)
				}
			}
			if chunk.Done {
				return finishWith(RunSucceeded, chat.MessageCompleted, "", "")
			}
		}
	}
}

func (s *Service) watchCancel(ctx context.Context, runID int64, cancel context.CancelFunc, once *sync.Once, hit chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requested, err := s.repo.CancelRequested(ctx, runID)
			if err == nil && requested {
				once.Do(func() {
					close(hit)
					cancel()
				})
				return
			}
		}
	}
}

func isCanceled(ctx context.Context, hit chan struct{}) bool {
	select {
	case <-hit:
		return true
	default:
		return false
	}
}

func (s *Service) failBeforeStream(ctx context.Context, run Run, cause error) error {
	assistant, _ := s.chat.MessageByRunID(ctx, run.ID)
	return s.finish(ctx, run, assistant, nil, RunFailed, chat.MessageFailed, "run_failed", cause.Error(), assistant.Content, nil)
}

func (s *Service) finish(
	ctx context.Context, run Run, assistant chat.Message, usage *ChatUsage,
	status RunStatus, messageStatus chat.MessageStatus, code, message, content string, firstTokenAt *time.Time,
) error {
	now := s.now().UTC()
	var amount int64
	billing := BillingUnbilled
	reason := "usage unavailable"
	if usage != nil {
		var snapshot PriceSnapshot
		if err := json.Unmarshal([]byte(run.PricingSnapshot), &snapshot); err != nil {
			reason = "pricing snapshot invalid"
		} else {
			calculated, calcErr := pricing.CalculateToken(pricing.TokenUsage{
				UncachedInput: usage.UncachedInput, CacheRead: usage.CacheRead,
				CacheWrite: usage.CacheWrite, Output: usage.Output,
			}, pricing.TokenPrices{
				Input: snapshot.InputCNYMicros, Output: snapshot.OutputCNYMicros,
				CacheRead: snapshot.CacheReadCNYMicros, CacheWrite: snapshot.CacheWriteCNY,
			}, snapshot.RatePPM)
			if calcErr != nil {
				reason = calcErr.Error()
			} else {
				walletUnits, convertErr := pricingMicrosToWalletUnits(calculated)
				if convertErr != nil {
					reason = convertErr.Error()
				} else {
					amount, billing, reason = walletUnits, BillingSettled, ""
				}
			}
		}
	}

	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked, err := s.repo.LockRunTx(ctx, tx, run.ID)
		if err != nil {
			return err
		}
		if locked.BillingStatus == BillingSettled || locked.BillingStatus == BillingUnbilled {
			return nil
		}
		var before, after *int64
		if billing == BillingSettled {
			_, current, err := s.wallet.DebitTx(ctx, tx, locked.UserID, amount, now)
			if err != nil {
				return err
			}
			value := current - amount
			before, after = &current, &value
		}
		chargeStatus := int16(1)
		if billing != BillingSettled {
			chargeStatus = 2
		}
		charge := UsageCharge{
			RunID: locked.ID, PlatformID: locked.PlatformID, UserID: locked.UserID,
			ProviderID: locked.ProviderID, ProviderModelID: locked.ProviderModelID,
			Status: chargeStatus, RatePPM: lockedTimeoutRate(locked.PricingSnapshot),
			PricingSnapshot: locked.PricingSnapshot, AmountUnits: amount,
			BalanceBeforeUnits: before, BalanceAfterUnits: after,
			UnbilledReason: reason, CreatedAt: now,
		}
		if usage != nil {
			charge.InputTokens = usage.UncachedInput
			charge.CachedInputTokens = usage.CacheRead
			charge.CacheWriteTokens = usage.CacheWrite
			charge.OutputTokens = usage.Output
		}
		if err := tx.WithContext(ctx).Create(&charge).Error; err != nil {
			return err
		}
		duration := int32(now.Sub(run.CreatedAt).Milliseconds())
		if duration < 0 {
			duration = 0
		}
		var firstTokenMS *int32
		if firstTokenAt != nil {
			value := int32(firstTokenAt.Sub(run.CreatedAt).Milliseconds())
			if value < 0 {
				value = 0
			}
			firstTokenMS = &value
		}
		updates := map[string]any{
			"status": status, "billing_status": billing, "amount_units": amount,
			"balance_before_units": before, "balance_after_units": after,
			"error_code": code, "error_message": truncate(message, 1024),
			"first_token_ms": firstTokenMS, "duration_ms": duration,
			"finished_at": now, "updated_at": now,
		}
		if usage != nil {
			updates["input_tokens"] = usage.UncachedInput
			updates["cached_input_tokens"] = usage.CacheRead
			updates["cache_write_tokens"] = usage.CacheWrite
			updates["output_tokens"] = usage.Output
		}
		if err := tx.WithContext(ctx).Model(&Run{}).Where("id = ?", locked.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&chat.Message{}).Where("id = ?", assistant.ID).
			Updates(map[string]any{"content": content, "status": messageStatus, "updated_at": now}).Error; err != nil {
			return err
		}
		eventType := "completed"
		eventMessage := "运行完成"
		if status != RunSucceeded {
			eventType, eventMessage = "failed", truncate(message, 500)
		}
		return s.repo.AppendRunEventTx(ctx, tx, locked.ID, eventType, eventMessage, "{}", now)
	})
	if txErr != nil {
		s.logger.Error("settle chat run", "runId", run.ID, "error", txErr)
		now := s.now().UTC()
		_ = s.repo.MarkRunFailed(ctx, run.ID, RunFailed, BillingFailed, "settle_failed", "结算失败", now)
		_ = s.chat.FinishMessage(ctx, assistant.ID, content, chat.MessageFailed, now)
		s.publish(ctx, run, realtime.EventAIChatFailed, map[string]any{
			"runId": run.ID, "requestId": run.RequestID, "conversationId": run.ConversationID,
			"assistantMessageId": assistant.ID, "status": RunFailed, "billingStatus": BillingFailed,
			"amountUnits": int64(0), "errorCode": "settle_failed", "message": "结算失败",
			"balanceBeforeUnits": int64(0), "balanceAfterUnits": int64(0),
		})
		return txErr
	}
	fresh := run
	if current, findErr := s.repo.FindRun(ctx, run.ID); findErr == nil {
		fresh = current
	}
	data := map[string]any{
		"runId": run.ID, "requestId": run.RequestID, "conversationId": run.ConversationID,
		"assistantMessageId": assistant.ID, "status": status, "billingStatus": billing,
		"amountUnits": amount, "errorCode": code, "message": message,
		"balanceBeforeUnits": derefInt64Ptr(fresh.BalanceBeforeUnits),
		"balanceAfterUnits":  derefInt64Ptr(fresh.BalanceAfterUnits),
	}
	eventType := realtime.EventAIChatCompleted
	if status == RunCanceled {
		eventType = realtime.EventAIChatCanceled
	} else if status != RunSucceeded {
		eventType = realtime.EventAIChatFailed
	}
	s.publish(ctx, run, eventType, data)
	return nil
}

func (s *Service) appendEvent(ctx context.Context, runID int64, eventType, message string, data any) error {
	payload, _ := json.Marshal(data)
	now := s.now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.repo.AppendRunEventTx(ctx, tx, runID, eventType, message, string(payload), now)
	})
}

func (s *Service) publish(ctx context.Context, run Run, eventType string, data any) {
	if s.publisher == nil {
		return
	}
	if err := realtime.PublishEphemeral(ctx, s.publisher, run.PlatformID, run.UserID, eventType, data, s.now().UTC()); err != nil {
		s.logger.Warn("publish chat event", "runId", run.ID, "type", eventType, "error", err)
	}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func derefInt64Ptr(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// pricing.CalculateToken returns CNY micros (1e6 per RMB), while the wallet
// stores units with 1e8 per RMB. Convert once at the settlement boundary.
func pricingMicrosToWalletUnits(value int64) (int64, error) {
	const microsPerRMB int64 = 1_000_000
	if value < 0 || value > math.MaxInt64/(money.UnitsPerRMB/microsPerRMB) {
		return 0, pricing.ErrOverflow
	}
	return value * (money.UnitsPerRMB / microsPerRMB), nil
}

func lockedTimeoutRate(snapshot string) int64 {
	var parsed PriceSnapshot
	if err := json.Unmarshal([]byte(snapshot), &parsed); err != nil {
		return 1000000
	}
	if parsed.RatePPM <= 0 {
		return 1000000
	}
	return parsed.RatePPM
}
