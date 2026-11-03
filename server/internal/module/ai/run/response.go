package run

import (
	"strconv"
	"time"
)

// Response is the public Run projection. Snapshots, ownership fields and
// internal lease coordinates never leave the service boundary.
type Response struct {
	ConversationID    *string    `json:"conversationId"`
	StateVersion      int64      `json:"stateVersion"`
	NextEventSequence int64      `json:"nextEventSequence"`
	CanCancel         bool       `json:"canCancel"`
	ID                string     `json:"id"`
	AgentID           string     `json:"agentId"`
	Operation         string     `json:"operation"`
	ExecutionStatus   int16      `json:"executionStatus"`
	UsageStatus       int16      `json:"usageStatus"`
	BillingStatus     int16      `json:"billingStatus"`
	TransferStatus    int16      `json:"transferStatus"`
	EstimatedUnits    int64      `json:"estimatedUnits"`
	AcceptedAt        time.Time  `json:"acceptedAt"`
	FinishedAt        *time.Time `json:"finishedAt"`
	ErrorCode         *string    `json:"errorCode"`
	ErrorMessage      *string    `json:"errorMessage"`
}

func PublicResponse(m Model) Response {
	var conversation *string
	if m.ConversationID != nil {
		v := strconv.FormatInt(*m.ConversationID, 10)
		conversation = &v
	}
	return Response{ConversationID: conversation, StateVersion: m.StateVersion, NextEventSequence: m.NextEventSequence, CanCancel: m.ExecutionStatus >= 0 && m.ExecutionStatus <= 2 && m.CancelRequestedAt == nil,
		ID: strconv.FormatInt(m.ID, 10), AgentID: strconv.FormatInt(m.AgentID, 10), Operation: m.Operation,
		ExecutionStatus: m.ExecutionStatus, UsageStatus: m.UsageStatus, BillingStatus: m.BillingStatus,
		TransferStatus: m.TransferStatus, EstimatedUnits: m.EstimatedUnits, AcceptedAt: m.AcceptedAt,
		FinishedAt: m.FinishedAt, ErrorCode: m.ErrorCode, ErrorMessage: m.ErrorMessage,
	}
}
