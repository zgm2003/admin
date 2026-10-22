package redeemcode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

const maxBatchSize = 1000
const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type GenerateInput struct {
	RequestID   string
	AmountCents int64
	Quantity    int
	ExpiresAt   *time.Time
	Remark      string
}
type GenerateResult struct {
	BatchID int64    `json:"batchId"`
	BatchNo string   `json:"batchNo"`
	Codes   []string `json:"codes"`
}
type Value struct {
	ID        int64      `json:"id"`
	BatchID   int64      `json:"batchId"`
	BatchNo   string     `json:"batchNo"`
	Code      string     `json:"code"`
	Status    Status     `json:"status"`
	Amount    string     `json:"amount"`
	ExpiresAt *time.Time `json:"expiresAt"`
	UsedBy    *int64     `json:"usedBy"`
	UsedAt    *time.Time `json:"usedAt"`
	CreatedAt time.Time  `json:"createdAt"`
	Actions   Actions    `json:"actions"`
}
type Actions struct {
	Void   bool `json:"void"`
	Export bool `json:"export"`
}

type ExportValue struct {
	Value
}
type Redemption struct {
	TransactionID int64     `json:"transactionId"`
	TransactionNo string    `json:"transactionNo"`
	Amount        string    `json:"amount"`
	Balance       string    `json:"balance"`
	CreatedAt     time.Time `json:"createdAt"`
}
type Options struct {
	Limits   Limits         `json:"limits"`
	Defaults Defaults       `json:"defaults"`
	Statuses []StatusOption `json:"statuses"`
}
type Limits struct {
	MaxBatchQuantity  int    `json:"maxBatchQuantity"`
	MaxVoidQuantity   int    `json:"maxVoidQuantity"`
	MaxExportQuantity int    `json:"maxExportQuantity"`
	MaxCodeLength     int    `json:"maxCodeLength"`
	CodeLength        int    `json:"codeLength"`
	Remark            int    `json:"remark"`
	MinimumAmount     string `json:"minimumAmount"`
	MaximumAmount     string `json:"maximumAmount"`
	AmountScale       int    `json:"amountScale"`
}
type Defaults struct {
	Quantity int `json:"quantity"`
}
type StatusOption struct {
	Value Status `json:"value"`
	Label string `json:"label"`
}
type ListQuery struct {
	Page     int
	PageSize int
	BatchID  int64
	Status   *Status
}

func normalizeCode(raw string) (string, error) {
	if len(raw) > 128 {
		return "", fmt.Errorf("code length invalid")
	}
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.NewReplacer("-", "", " ", "", "\t", "", "\r", "", "\n", "").Replace(s)
	if len(s) != 20 {
		return "", fmt.Errorf("code length invalid")
	}
	for _, c := range s {
		if !strings.ContainsRune(alphabet, c) {
			return "", fmt.Errorf("code contains invalid character")
		}
	}
	return s, nil
}
func newCode() (string, error) {
	const n = byte(len(alphabet))
	limit := 256
	if remainder := 256 % int(n); remainder != 0 {
		limit -= remainder
	}
	out := make([]byte, 20)
	buf := make([]byte, 32)
	for filled := 0; filled < len(out); {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out[filled] = alphabet[int(b%n)]
			filled++
			if filled == len(out) {
				break
			}
		}
	}
	return string(out), nil
}

// Stored codes are canonical facts; malformed data must not be normalized silently.
func storedCode(code string) (string, error) {
	canonical, err := normalizeCode(code)
	if err != nil || canonical != code {
		return "", ErrInvariant
	}
	return code, nil
}
func fingerprint(in GenerateInput) []byte {
	payload, _ := json.Marshal(struct {
		AmountCents int64      `json:"amountCents"`
		Quantity    int        `json:"quantity"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		Remark      string     `json:"remark"`
	}{in.AmountCents, in.Quantity, in.ExpiresAt, in.Remark})
	h := sha256.Sum256(payload)
	return h[:]
}
func batchNo() (string, error) {
	b := make([]byte, 8)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return "RB" + strings.ToUpper(hex.EncodeToString(b)), nil
}
func requestUUID(raw string) (string, error) {
	id, e := uuid.Parse(raw)
	if e != nil {
		return "", fmt.Errorf("requestId invalid")
	}
	return id.String(), nil
}
