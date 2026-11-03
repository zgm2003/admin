package run

import (
	"admin/server/internal/module/ai"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var ErrQueryRangeTooLarge = errors.New("AI summary range exceeds candidate limit")

type QueryOwner struct{ UserID, PlatformID int64 }

func (o QueryOwner) valid() bool { return o.UserID > 0 && o.PlatformID > 0 }

type RunQuery struct {
	StartAt         time.Time `form:"startAt"`
	EndAt           time.Time `form:"endAt"`
	BeforeID        int64     `form:"beforeId"`
	Limit           int       `form:"limit"`
	UserID          int64     `form:"userId"`
	PlatformID      int64     `form:"platformId"`
	AgentID         int64     `form:"agentId"`
	ProviderID      int64     `form:"providerId"`
	ProviderModelID int64     `form:"providerModelId"`
	Operation       string    `form:"operation"`
	ExecutionStatus *int16    `form:"executionStatus"`
	UsageStatus     *int16    `form:"usageStatus"`
	BillingStatus   *int16    `form:"billingStatus"`
	TransferStatus  *int16    `form:"transferStatus"`
}

func (q *RunQuery) normalize(now time.Time, summary bool) error {
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.EndAt.IsZero() {
		q.EndAt = now
	}
	if q.StartAt.IsZero() {
		d := 24 * time.Hour
		if summary {
			d = time.Hour
		}
		q.StartAt = q.EndAt.Add(-d)
	}
	maximum := 31 * 24 * time.Hour
	if summary {
		maximum = 24 * time.Hour
	}
	if q.Limit < 1 || q.Limit > 100 || q.BeforeID < 0 || !q.StartAt.Before(q.EndAt) || q.EndAt.Sub(q.StartAt) > maximum || q.UserID < 0 || q.PlatformID < 0 || q.AgentID < 0 || q.ProviderID < 0 || q.ProviderModelID < 0 {
		return errors.New("invalid AI query range")
	}
	if q.Operation != "" && !ai.Operation(q.Operation).Valid() {
		return errors.New("invalid AI operation")
	}
	for _, s := range []*int16{q.ExecutionStatus, q.UsageStatus, q.BillingStatus, q.TransferStatus} {
		if s != nil && (*s < 0 || *s > 5) {
			return errors.New("invalid AI query status")
		}
	}
	if q.UsageStatus != nil && *q.UsageStatus > 2 {
		return errors.New("invalid usage status")
	}
	if q.BillingStatus != nil && *q.BillingStatus > 4 {
		return errors.New("invalid billing status")
	}
	return nil
}

type CursorPage[T any] struct {
	List         []T    `json:"list"`
	HasMore      bool   `json:"hasMore"`
	NextBeforeID string `json:"nextBeforeId"`
	NextSequence int64  `json:"nextSequence"`
	ResetReason  string `json:"resetReason,omitempty"`
}

func page[T any](rows []T, limit int, id func(T) int64) CursorPage[T] {
	v := CursorPage[T]{List: rows, NextBeforeID: ""}
	if len(rows) > limit {
		v.List = rows[:limit]
		v.HasMore = true
	}
	if len(v.List) > 0 {
		v.NextBeforeID = strconv.FormatInt(id(v.List[len(v.List)-1]), 10)
	}
	return v
}

type AdminRunResponse struct {
	Response
	UserID          string           `json:"userId"`
	PlatformID      string           `json:"platformId"`
	ProviderModelID string           `json:"providerModelId"`
	PriceVersionID  string           `json:"priceVersionId"`
	Configuration   RunConfiguration `json:"configuration"`
}
type RunConfiguration struct {
	BindingID         string `json:"bindingId"`
	ProviderVersionID string `json:"providerVersionId"`
	OfficialModelID   string `json:"officialModelId"`
	MultiplierPPM     int64  `json:"multiplierPpm"`
}

func AdminResponse(m Model) AdminRunResponse {
	return AdminRunResponse{Response: PublicResponse(m), UserID: strconv.FormatInt(m.UserID, 10), PlatformID: strconv.FormatInt(m.PlatformID, 10), ProviderModelID: strconv.FormatInt(m.ProviderModelID, 10), PriceVersionID: strconv.FormatInt(m.PriceVersionID, 10), Configuration: RunConfiguration{strconv.FormatInt(m.BindingID, 10), strconv.FormatInt(m.ProviderVersionID, 10), strconv.FormatInt(m.OfficialModelID, 10), m.MultiplierPPM}}
}

type QuerySummary struct {
	RunCount       int64  `json:"runCount"`
	EstimatedUnits string `json:"estimatedUnits"`
	SettledUnits   string `json:"settledUnits"`
}
type EventResponse struct {
	ID        string          `json:"id"`
	RunID     string          `json:"runId"`
	Sequence  int64           `json:"sequence"`
	EventType string          `json:"eventType"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}
type RequestResponse struct {
	ID            string          `json:"id"`
	RunID         string          `json:"runId"`
	Sequence      int             `json:"sequence"`
	Status        int16           `json:"status"`
	ResponseItems json.RawMessage `json:"responseItems"`
	Usage         json.RawMessage `json:"usage"`
	UsageStatus   int16           `json:"usageStatus"`
	StartedAt     *time.Time      `json:"startedAt"`
	FinishedAt    *time.Time      `json:"finishedAt"`
}
type ToolResponse struct {
	ID                string     `json:"id"`
	RunID             string     `json:"runId"`
	ProviderRequestID string     `json:"providerRequestId"`
	ExposedName       string     `json:"exposedName"`
	Status            int16      `json:"status"`
	StartedAt         *time.Time `json:"startedAt"`
	FinishedAt        *time.Time `json:"finishedAt"`
}
type ChargeResponse struct {
	ID             string     `json:"id"`
	RunID          string     `json:"runId"`
	Status         int16      `json:"status"`
	Currency       string     `json:"currency"`
	PriceVersionID string     `json:"priceVersionId"`
	MultiplierPPM  int64      `json:"multiplierPpm"`
	ActualUnits    *int64     `json:"actualUnits,string"`
	UsageComplete  int16      `json:"usageComplete"`
	SettledAt      *time.Time `json:"settledAt"`
	TransactionID  *string    `json:"transactionId"`
}
type ChargeItemResponse struct {
	ID                    string          `json:"id"`
	ChargeID              string          `json:"chargeId"`
	ProviderRequestID     string          `json:"providerRequestId"`
	PriceRateID           string          `json:"priceRateId"`
	Operation             string          `json:"operation"`
	UsageKey              string          `json:"usageKey"`
	Selector              json.RawMessage `json:"selector"`
	Quantity              string          `json:"quantity"`
	UnitQuantity          string          `json:"unitQuantity"`
	Unit                  string          `json:"unit"`
	BillingUnitPriceUnits int64           `json:"billingUnitPriceUnits,string"`
	MultiplierPPM         int64           `json:"multiplierPpm"`
	AmountUnits           int64           `json:"amountUnits,string"`
}
type MediaResponse struct {
	ErrorCode        *string `json:"errorCode"`
	ErrorMessage     *string `json:"errorMessage"`
	ID               string  `json:"id"`
	RunID            string  `json:"runId"`
	Role             int16   `json:"role"`
	Ordinal          int     `json:"ordinal"`
	StorageFileID    *string `json:"storageFileId"`
	MIMEType         *string `json:"mimeType"`
	SizeBytes        *int64  `json:"sizeBytes"`
	TransferStatus   int16   `json:"transferStatus"`
	CanPreview       bool    `json:"canPreview"`
	CanRetryTransfer bool    `json:"canRetryTransfer"`
}

type DetailResponse struct {
	Response
	Input  RunInput        `json:"input"`
	Result json.RawMessage `json:"result"`
}
type AdminDetailResponse struct {
	AdminRunResponse
	Input  RunInput        `json:"input"`
	Result json.RawMessage `json:"result"`
}
