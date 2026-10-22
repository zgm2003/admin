package redeemcode

import "time"

type Status int16

const (
	StatusUnused  Status = 1
	StatusUsed    Status = 2
	StatusVoided  Status = 3
	StatusExpired Status = 4
)

type Batch struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	BatchNo            string     `gorm:"column:batch_no"`
	RequestID          string     `gorm:"column:request_id"`
	RequestFingerprint []byte     `gorm:"column:request_fingerprint"`
	AmountCents        int64      `gorm:"column:amount_cents"`
	Quantity           int        `gorm:"column:quantity"`
	ExpiresAt          *time.Time `gorm:"column:expires_at"`
	Remark             *string    `gorm:"column:remark"`
	CreatedBy          int64      `gorm:"column:created_by"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (Batch) TableName() string { return "payment_redeem_code_batch" }

type Model struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	BatchID     int64      `gorm:"column:batch_id"`
	Code        string     `gorm:"column:code"`
	Status      Status     `gorm:"column:status"`
	UsedBy      *int64     `gorm:"column:used_by"`
	UsedAt      *time.Time `gorm:"column:used_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	ExpiresAt   *time.Time `gorm:"->;column:expires_at"`
	BatchNo     string     `gorm:"->;column:batch_no"`
	AmountCents int64      `gorm:"->;column:amount_cents"`
	Remark      *string    `gorm:"->;column:remark"`
}

func (Model) TableName() string { return "payment_redeem_code" }

func projectedStatus(m Model, now time.Time) Status {
	if m.Status == StatusUnused && m.ExpiresAt != nil && !m.ExpiresAt.After(now) {
		return StatusExpired
	}
	return m.Status
}
