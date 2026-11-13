package chat

import (
	"context"
	"errors"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment/wallet"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/pagination"
)

const defaultConversationTitle = "新对话"

type Service struct {
	repo   *Repository
	wallet *wallet.Repository
	now    func() time.Time
}

func NewService(repo *Repository, walletRepository *wallet.Repository) *Service {
	return &Service{repo: repo, wallet: walletRepository, now: time.Now}
}

func (s *Service) Agents(ctx context.Context) ([]AgentValue, error) {
	if s == nil || s.repo == nil {
		return nil, dependency(errors.New("chat service is not configured"))
	}
	rows, err := s.repo.ListAgents(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]AgentValue, 0, len(rows))
	for _, row := range rows {
		items = append(items, agentValue(row))
	}
	return items, nil
}

func (s *Service) Conversations(ctx context.Context, identity auth.Identity, query ListConversationsQuery) (pagination.Result[ConversationValue], error) {
	if s == nil || s.repo == nil {
		return pagination.Result[ConversationValue]{}, dependency(errors.New("chat service is not configured"))
	}
	rows, total, err := s.repo.ListConversations(ctx, identity.PlatformID, identity.UserID, query.Keyword,
		(query.Page-1)*query.PageSize, query.PageSize)
	if err != nil {
		return pagination.Result[ConversationValue]{}, mapError(err)
	}
	items := make([]ConversationValue, 0, len(rows))
	for _, row := range rows {
		items = append(items, conversationValue(row))
	}
	return pagination.Result[ConversationValue]{
		List: items, Total: total, Page: query.Page, PageSize: query.PageSize,
	}, nil
}

func (s *Service) CreateConversation(ctx context.Context, identity auth.Identity, input CreateConversationInput) (int64, error) {
	if _, err := s.repo.FindAgent(ctx, input.AgentID); err != nil {
		return 0, mapError(err)
	}
	title := input.Title
	if title == "" {
		title = defaultConversationTitle
	}
	now := s.now().UTC()
	return s.repo.CreateConversation(ctx, Conversation{
		PlatformID: identity.PlatformID, UserID: identity.UserID, AgentID: input.AgentID,
		Title: title, LastMessageAt: now, MessageCount: 0, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (s *Service) RenameConversation(ctx context.Context, identity auth.Identity, id int64, input RenameConversationInput) error {
	if _, err := s.repo.FindConversation(ctx, identity.PlatformID, identity.UserID, id); err != nil {
		return mapError(err)
	}
	return mapError(s.repo.UpdateConversationTitle(ctx, identity.PlatformID, identity.UserID, id, input.Title, s.now().UTC()))
}

func (s *Service) DeleteConversation(ctx context.Context, identity auth.Identity, id int64) error {
	return mapError(s.repo.DeleteConversation(ctx, identity.PlatformID, identity.UserID, id, s.now().UTC()))
}

func (s *Service) Messages(ctx context.Context, identity auth.Identity, conversationID int64, beforeSeq int32, limit int) (MessagePage, error) {
	conversation, err := s.repo.FindConversation(ctx, identity.PlatformID, identity.UserID, conversationID)
	if err != nil {
		return MessagePage{}, mapError(err)
	}
	rows, err := s.repo.ListMessages(ctx, conversation.ID, beforeSeq, limit)
	if err != nil {
		return MessagePage{}, mapError(err)
	}
	messageIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		messageIDs = append(messageIDs, row.ID)
	}
	attachments, err := s.repo.AttachmentsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return MessagePage{}, mapError(err)
	}
	items := make([]MessageValue, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		row.Attachments = attachments[row.ID]
		items = append(items, messageValue(row))
	}
	page := MessagePage{List: items, HasMore: len(rows) == limit}
	active, exists, err := s.repo.ActiveRunSummary(ctx, conversation.ID)
	if err != nil {
		return MessagePage{}, mapError(err)
	}
	if exists {
		page.ActiveRun = &ActiveRunValue{
			ID: active.ID, RequestID: active.RequestID, Status: active.Status,
			BillingStatus: active.BillingStatus, AssistantMessageID: active.AssistantMessageID,
			DeliverySeq: active.DeliverySeq,
		}
	}
	return page, nil
}

func (s *Service) Feedback(ctx context.Context, identity auth.Identity, messageID int64, feedback int16) (int16, error) {
	if s == nil || s.repo == nil {
		return 0, dependency(errors.New("chat service is not configured"))
	}
	if messageID < 1 || (feedback != -1 && feedback != 0 && feedback != 1) {
		return 0, invalid(errors.New("feedback input invalid"))
	}
	found, err := s.repo.SetMessageFeedback(ctx, identity.PlatformID, identity.UserID, messageID, feedback, s.now().UTC())
	if err != nil {
		return 0, mapError(err)
	}
	if !found {
		return 0, mapError(ErrMessageNotFound)
	}
	return feedback, nil
}

func (s *Service) Balance(ctx context.Context, userID int64) (BalanceValue, error) {
	row, exists, err := s.wallet.Find(ctx, userID)
	if err != nil {
		return BalanceValue{}, dependency(err)
	}
	balance := int64(0)
	if exists {
		balance = row.BalanceUnits
	}
	formatted, _ := money.FormatRMBUnits(balance)
	return BalanceValue{BalanceUnits: balance, Balance: formatted, CanSend: balance > 0}, nil
}
