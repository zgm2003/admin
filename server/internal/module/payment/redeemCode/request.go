package redeemcode

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"admin/server/internal/shared/money"
	"admin/server/internal/shared/pagination"
)

type generateRequest struct {
	RequestID string     `json:"requestId"`
	Amount    string     `json:"amount"`
	Quantity  int        `json:"quantity"`
	ExpiresAt *time.Time `json:"expiresAt"`
	Remark    string     `json:"remark"`
}

func (r generateRequest) input() (GenerateInput, error) {
	cents, e := money.ParseRMBCents(r.Amount)
	if e != nil || cents < 1 {
		return GenerateInput{}, ErrInvalidInput
	}
	return GenerateInput{RequestID: r.RequestID, AmountCents: cents, Quantity: r.Quantity, ExpiresAt: r.ExpiresAt, Remark: r.Remark}, nil
}
func parseListQuery(v url.Values) (ListQuery, error) {
	q := ListQuery{}
	for k, e := range v {
		if len(e) != 1 {
			return q, ErrInvalidInput
		}
		switch k {
		case "page", "pageSize", "batchId", "status":
		default:
			return q, ErrInvalidInput
		}
	}
	p, e := pagination.ParseRequest(v)
	if e != nil {
		return q, e
	}
	q.Page, q.PageSize = p.Page, p.PageSize
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || (q.Page-1) > int(^uint(0)>>1)/q.PageSize {
		return q, ErrInvalidInput
	}
	if raw, ok := v["batchId"]; ok {
		id, e := positiveDecimal(raw[0])
		if e != nil {
			return q, e
		}
		q.BatchID = id
	}
	if raw, ok := v["status"]; ok {
		id, e := positiveDecimal(raw[0])
		if e != nil || id > 4 {
			return q, ErrInvalidInput
		}
		s := Status(id)
		q.Status = &s
	}
	return q, nil
}
func positiveDecimal(raw string) (int64, error) {
	if raw == "" {
		return 0, ErrInvalidInput
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, ErrInvalidInput
		}
	}
	n, e := strconv.ParseInt(raw, 10, 64)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%w: positive integer expected", ErrInvalidInput)
	}
	return n, nil
}
