package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/ai/chat"
	"admin/server/internal/module/ai/pricing"
	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment/wallet"
	"admin/server/internal/module/realtime"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/pagination"

	"gorm.io/gorm"
)

type AttachmentURLResolver interface {
	ObjectURL(ctx context.Context, identity auth.Identity, objectKey, disposition string) (storagefile.URLResult, error)
}

type Service struct {
	repo           *Repository
	chat           *chat.Repository
	db             *gorm.DB
	wallet         *wallet.Repository
	keys           *secretkey.KeyRing
	publisher      realtime.RawPublisher
	provider       *ProviderClient
	logger         *slog.Logger
	attachmentURLs AttachmentURLResolver
	now            func() time.Time
}

func NewService(
	repo *Repository,
	chatRepository *chat.Repository,
	db *gorm.DB,
	walletRepository *wallet.Repository,
	keys *secretkey.KeyRing,
	publisher realtime.RawPublisher,
	provider *ProviderClient,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if provider == nil {
		provider = NewProviderClient(nil)
	}
	return &Service{
		repo: repo, chat: chatRepository, db: db, wallet: walletRepository,
		keys: keys, publisher: publisher, provider: provider, logger: logger, now: time.Now,
	}
}

func (s *Service) SetAttachmentURLResolver(resolver AttachmentURLResolver) *Service {
	if s != nil {
		s.attachmentURLs = resolver
	}
	return s
}

type SendResult struct {
	RunID            int64             `json:"runId"`
	UserMessage      chat.MessageValue `json:"userMessage"`
	AssistantMessage chat.MessageValue `json:"assistantMessage"`
}

func (s *Service) Send(ctx context.Context, identity auth.Identity, conversationID int64, input SendInput) (SendResult, error) {
	if existing, ok, err := s.repo.FindRunByRequest(ctx, identity.PlatformID, identity.UserID, input.RequestID); err != nil {
		return SendResult{}, mapError(err)
	} else if ok {
		return s.existingSendResult(ctx, existing)
	}
	conversation, err := s.chat.FindConversation(ctx, identity.PlatformID, identity.UserID, conversationID)
	if err != nil {
		return SendResult{}, mapError(err)
	}
	agent, err := s.repo.FindAgent(ctx, conversation.AgentID)
	if err != nil {
		return SendResult{}, mapError(err)
	}
	if agent.AgentType != int16(pricing.KindChat) || agent.Scene != 1 {
		return SendResult{}, invalid(errors.New("conversation agent is not a conversation chat agent"))
	}
	walletRow, exists, err := s.wallet.Find(ctx, identity.UserID)
	if err != nil {
		return SendResult{}, dependency(err)
	}
	if !exists || walletRow.BalanceUnits <= 0 {
		return SendResult{}, insufficient(errors.New("wallet balance is not positive"))
	}
	objectKeys := make([]string, 0, len(input.Attachments))
	for _, attachment := range input.Attachments {
		objectKeys = append(objectKeys, attachment.ObjectKey)
	}
	fileRefs, err := s.chat.FindReadyFilesByObjectKeys(ctx, identity.PlatformID, identity.UserID, objectKeys)
	if err != nil {
		return SendResult{}, dependency(err)
	}
	if len(fileRefs) != len(objectKeys) {
		return SendResult{}, invalid(errors.New("attachment is unavailable"))
	}
	attachments := make([]chat.MessageAttachment, 0, len(fileRefs))
	fileByKey := make(map[string]chat.StorageFileRef, len(fileRefs))
	for _, file := range fileRefs {
		fileByKey[file.ObjectKey] = file
	}
	for _, objectKey := range objectKeys {
		file := fileByKey[objectKey]
		kind := chat.AttachmentFile
		if strings.HasPrefix(strings.ToLower(file.MimeType), "image/") {
			kind = chat.AttachmentImage
		}
		attachments = append(attachments, chat.MessageAttachment{
			Kind: kind, ObjectKey: file.ObjectKey, FileName: file.FileName,
			MimeType: file.MimeType, SizeBytes: file.SizeBytes,
		})
	}
	snapshot, err := json.Marshal(PriceSnapshot{
		Currency: "CNY", InputCNYMicros: agent.InputCNYMicros, OutputCNYMicros: agent.OutputCNYMicros,
		CacheReadCNYMicros: agent.CacheReadCNYMicros, CacheWriteCNY: agent.CacheWriteCNY,
		RatePPM: agent.TokenRatePPM,
	})
	if err != nil {
		return SendResult{}, dependency(err)
	}
	now := s.now().UTC()
	run := Run{
		RequestID: input.RequestID, PlatformID: identity.PlatformID, UserID: identity.UserID,
		AgentID: agent.ID, ProviderID: agent.ProviderID, ProviderModelID: agent.ProviderModelID,
		ProviderProtocol: agent.ProviderProtocol, AgentName: agent.Name,
		ProviderName: agent.ProviderName, ModelKey: agent.ModelKey, ModelName: agent.ModelName,
		TimeoutMS: agent.TimeoutMS, PricingSnapshot: string(snapshot),
		Status: RunQueued, BillingStatus: BillingPending, CreatedAt: now, UpdatedAt: now,
	}
	title := titleFromContent(input.Content)
	if title == "" && len(attachments) > 0 {
		title = titleFromContent(attachments[0].FileName)
	}
	var userMessage, assistant chat.Message
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		userMessage, assistant, txErr = s.repo.CreateTurnTx(
			ctx, tx, conversation, input.Content, title, attachments, run, now,
		)
		return txErr
	})
	if err != nil {
		return SendResult{}, mapError(err)
	}
	s.StartRun(assistant.RunIDValue())
	return SendResult{
		RunID: *assistant.RunID, UserMessage: messageValue(userMessage),
		AssistantMessage: messageValue(assistant),
	}, nil
}

func (s *Service) existingSendResult(ctx context.Context, run Run) (SendResult, error) {
	assistant, err := s.chat.MessageByRunID(ctx, run.ID)
	if err != nil {
		return SendResult{RunID: run.ID}, nil
	}
	preceding, err := s.chat.ListMessages(ctx, run.ConversationID, assistant.Seq, 1)
	if err != nil {
		return SendResult{}, mapError(err)
	}
	var userMessage chat.Message
	if len(preceding) == 1 && preceding[0].Role == chat.RoleUser {
		userMessage = preceding[0]
	}
	messageIDs := []int64{assistant.ID}
	if userMessage.ID > 0 {
		messageIDs = append(messageIDs, userMessage.ID)
	}
	attachments, err := s.chat.AttachmentsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return SendResult{}, mapError(err)
	}
	assistant.Attachments = attachments[assistant.ID]
	userMessage.Attachments = attachments[userMessage.ID]
	result := SendResult{RunID: run.ID, AssistantMessage: messageValue(assistant)}
	if userMessage.ID > 0 {
		result.UserMessage = messageValue(userMessage)
	}
	return result, nil
}

func (s *Service) Cancel(ctx context.Context, identity auth.Identity, runID int64) error {
	return mapError(s.repo.CancelRun(ctx, identity.PlatformID, identity.UserID, runID, s.now().UTC()))
}

func (s *Service) Runs(ctx context.Context, identity auth.Identity, query RunListQuery) (pagination.Result[RunValue], error) {
	query.PlatformID = identity.PlatformID
	query.SelfUserID = identity.UserID
	if !query.ViewAll {
		query.UserID = nil
	}
	rows, total, err := s.repo.ListRuns(ctx, query)
	if err != nil {
		return pagination.Result[RunValue]{}, mapError(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	names, err := s.repo.Usernames(ctx, ids)
	if err != nil {
		return pagination.Result[RunValue]{}, mapError(err)
	}
	items := make([]RunValue, 0, len(rows))
	for _, row := range rows {
		items = append(items, runValue(row, names[row.UserID]))
	}
	return pagination.Result[RunValue]{
		List: items, Total: total, Page: query.Page, PageSize: query.PageSize,
	}, nil
}

func (s *Service) RunDetail(ctx context.Context, identity auth.Identity, runID int64, viewAll bool) (RunDetail, error) {
	run, err := s.repo.FindRun(ctx, runID)
	if err != nil {
		return RunDetail{}, mapError(err)
	}
	if !viewAll && (run.PlatformID != identity.PlatformID || run.UserID != identity.UserID) {
		return RunDetail{}, mapError(ErrRunNotFound)
	}
	events, err := s.repo.ListRunEvents(ctx, runID)
	if err != nil {
		return RunDetail{}, mapError(err)
	}
	username, _ := s.repo.Username(ctx, run.UserID)
	eventValues := make([]RunEventValue, 0, len(events))
	for _, event := range events {
		eventValues = append(eventValues, RunEventValue{
			Seq: event.Seq, Type: event.Type, Message: event.Message, CreatedAt: event.CreatedAt,
		})
	}
	detail := RunDetail{RunValue: runValue(run, username), Events: eventValues}
	userMessage, assistantMessage, messageErr := s.repo.TurnMessages(ctx, runID)
	if messageErr != nil && !errors.Is(messageErr, gorm.ErrRecordNotFound) {
		return RunDetail{}, mapError(messageErr)
	}
	if messageErr == nil {
		attachments, attachErr := s.chat.AttachmentsByMessageIDs(ctx, []int64{userMessage.ID, assistantMessage.ID})
		if attachErr != nil {
			return RunDetail{}, mapError(attachErr)
		}
		userMessage.Attachments = attachments[userMessage.ID]
		assistantMessage.Attachments = attachments[assistantMessage.ID]
		userValue, assistantValue := runMessageValue(userMessage), runMessageValue(assistantMessage)
		detail.UserMessage, detail.AssistantMessage = &userValue, &assistantValue
	}
	return detail, nil
}

func (s *Service) StartRun(runID int64) {
	if runID < 1 {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("chat run panic", "runId", runID, "panic", fmt.Sprint(recovered))
			}
		}()
		if err := s.Execute(context.Background(), runID); err != nil {
			s.logger.Error("chat run failed", "runId", runID, "error", err)
		}
	}()
}

func (s *Service) RecoverInterrupted(ctx context.Context) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	return s.repo.RecoverInterrupted(ctx, s.now().UTC())
}

func (s *Service) decryptAPIKey(agent AgentConfig) (string, error) {
	if s.keys == nil {
		return "", errors.New("chat key ring is not configured")
	}
	return secretkey.DecryptProviderValue(s.keys.ProviderEncryptionKey(), agent.APIKeyCiphertext)
}

func titleFromContent(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	if utf8.RuneCountInString(content) > 60 {
		runes := []rune(content)
		content = string(runes[:60]) + "…"
	}
	return content
}
