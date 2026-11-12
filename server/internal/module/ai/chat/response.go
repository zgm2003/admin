package chat

import "time"

type AgentValue struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Scene        int16  `json:"scene"`
	ProviderName string `json:"providerName"`
	ModelName    string `json:"modelName"`
	TimeoutMS    int32  `json:"timeoutMs"`
}

type ConversationValue struct {
	ID            int64     `json:"id"`
	AgentID       int64     `json:"agentId"`
	AgentName     string    `json:"agentName"`
	Title         string    `json:"title"`
	ModelName     string    `json:"modelName"`
	MessageCount  int32     `json:"messageCount"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type MessageValue struct {
	ID          int64         `json:"id"`
	Seq         int32         `json:"seq"`
	Role        MessageRole   `json:"role"`
	Content     string        `json:"content"`
	Status      MessageStatus `json:"status"`
	RunID       *int64        `json:"runId"`
	DeliverySeq int64         `json:"deliverySeq"`
	CreatedAt   time.Time     `json:"createdAt"`
}

type ActiveRunValue struct {
	ID                 int64  `json:"id"`
	RequestID          string `json:"requestId"`
	Status             int16  `json:"status"`
	BillingStatus      int16  `json:"billingStatus"`
	AssistantMessageID int64  `json:"assistantMessageId"`
	DeliverySeq        int64  `json:"deliverySeq"`
}

type MessagePage struct {
	List      []MessageValue  `json:"list"`
	HasMore   bool            `json:"hasMore"`
	ActiveRun *ActiveRunValue `json:"activeRun,omitempty"`
}

type BalanceValue struct {
	BalanceUnits int64  `json:"balanceUnits"`
	Balance      string `json:"balance"`
	CanSend      bool   `json:"canSend"`
}

func conversationValue(row ConversationView) ConversationValue {
	return ConversationValue{
		ID: row.ID, AgentID: row.AgentID, AgentName: row.AgentName, Title: row.Title,
		ModelName: row.ModelName, MessageCount: row.MessageCount,
		LastMessageAt: row.LastMessageAt, UpdatedAt: row.UpdatedAt,
	}
}

func messageValue(row Message) MessageValue {
	return MessageValue{
		ID: row.ID, Seq: row.Seq, Role: row.Role, Content: row.Content, Status: row.Status,
		RunID: row.RunID, DeliverySeq: row.DeliverySeq, CreatedAt: row.CreatedAt,
	}
}

func agentValue(row AgentRow) AgentValue {
	return AgentValue{
		ID: row.ID, Name: row.Name, Description: row.Description, Scene: row.Scene,
		ProviderName: row.ProviderName, ModelName: row.ModelName, TimeoutMS: row.TimeoutMS,
	}
}
