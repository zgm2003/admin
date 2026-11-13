package run

import (
	"time"

	"admin/server/internal/module/ai/chat"
	"admin/server/internal/shared/money"
)

type RunValue struct {
	ID                int64         `json:"id"`
	RequestID         string        `json:"requestId"`
	UserID            int64         `json:"userId"`
	Username          string        `json:"username"`
	ConversationID    int64         `json:"conversationId"`
	AgentID           int64         `json:"agentId"`
	AgentName         string        `json:"agentName"`
	ProviderName      string        `json:"providerName"`
	ModelKey          string        `json:"modelKey"`
	ModelName         string        `json:"modelName"`
	Status            RunStatus     `json:"status"`
	BillingStatus     BillingStatus `json:"billingStatus"`
	ErrorCode         string        `json:"errorCode"`
	ErrorMessage      string        `json:"errorMessage"`
	InputTokens       int64         `json:"inputTokens"`
	CachedInputTokens int64         `json:"cachedInputTokens"`
	OutputTokens      int64         `json:"outputTokens"`
	Amount            string        `json:"amount"`
	AmountUnits       int64         `json:"amountUnits"`
	BalanceBefore     string        `json:"balanceBefore"`
	BalanceAfter      string        `json:"balanceAfter"`
	FirstTokenMS      *int32        `json:"firstTokenMs"`
	DurationMS        *int32        `json:"durationMs"`
	CreatedAt         time.Time     `json:"createdAt"`
	StartedAt         *time.Time    `json:"startedAt"`
	FinishedAt        *time.Time    `json:"finishedAt"`
}

type RunEventValue struct {
	Seq       int32     `json:"seq"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type RunMessageValue struct {
	ID          int64                `json:"id"`
	Role        int16                `json:"role"`
	Content     string               `json:"content"`
	Status      int16                `json:"status"`
	Feedback    int16                `json:"feedback"`
	CreatedAt   time.Time            `json:"createdAt"`
	Attachments []RunAttachmentValue `json:"attachments"`
}

type RunAttachmentValue struct {
	Kind      int16  `json:"kind"`
	ObjectKey string `json:"objectKey"`
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
}

type RunDetail struct {
	RunValue
	UserMessage      *RunMessageValue `json:"userMessage,omitempty"`
	AssistantMessage *RunMessageValue `json:"assistantMessage,omitempty"`
	Events           []RunEventValue  `json:"events"`
}

func runMessageValue(row chat.Message) RunMessageValue {
	value := RunMessageValue{
		ID: row.ID, Role: int16(row.Role), Content: row.Content, Status: int16(row.Status),
		Feedback: row.Feedback, CreatedAt: row.CreatedAt,
		Attachments: make([]RunAttachmentValue, 0, len(row.Attachments)),
	}
	for _, attachment := range row.Attachments {
		value.Attachments = append(value.Attachments, RunAttachmentValue{
			Kind: int16(attachment.Kind), ObjectKey: attachment.ObjectKey, FileName: attachment.FileName,
			MimeType: attachment.MimeType, SizeBytes: attachment.SizeBytes,
		})
	}
	return value
}

func messageValue(row chat.Message) chat.MessageValue {
	value := chat.MessageValue{
		ID: row.ID, Seq: row.Seq, Role: row.Role, Content: row.Content, Status: row.Status,
		RunID: row.RunID, DeliverySeq: row.DeliverySeq, Feedback: row.Feedback,
		CreatedAt: row.CreatedAt, Attachments: make([]chat.AttachmentValue, 0, len(row.Attachments)),
	}
	for _, attachment := range row.Attachments {
		value.Attachments = append(value.Attachments, chat.AttachmentValue{
			Kind: attachment.Kind, ObjectKey: attachment.ObjectKey, FileName: attachment.FileName,
			MimeType: attachment.MimeType, SizeBytes: attachment.SizeBytes,
		})
	}
	return value
}

func runValue(row Run, username string) RunValue {
	amount, _ := money.FormatRMBUnits(row.AmountUnits)
	before, after := "", ""
	if row.BalanceBeforeUnits != nil {
		before, _ = money.FormatRMBUnits(*row.BalanceBeforeUnits)
	}
	if row.BalanceAfterUnits != nil {
		after, _ = money.FormatRMBUnits(*row.BalanceAfterUnits)
	}
	return RunValue{
		ID: row.ID, RequestID: row.RequestID, UserID: row.UserID, Username: username,
		ConversationID: row.ConversationID, AgentID: row.AgentID, AgentName: row.AgentName,
		ProviderName: row.ProviderName, ModelKey: row.ModelKey, ModelName: row.ModelName,
		Status: row.Status, BillingStatus: row.BillingStatus,
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage,
		InputTokens: row.InputTokens, CachedInputTokens: row.CachedInputTokens,
		OutputTokens: row.OutputTokens, Amount: amount, AmountUnits: row.AmountUnits,
		BalanceBefore: before, BalanceAfter: after,
		FirstTokenMS: row.FirstTokenMS, DurationMS: row.DurationMS,
		CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
	}
}
