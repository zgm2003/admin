package transaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type SourceType int16

const (
	SourcePayment    SourceType = 1
	SourceRedeemCode SourceType = 2
)

func (v SourceType) Valid() bool { return v == SourcePayment || v == SourceRedeemCode }

type repository interface {
	Find(context.Context, int64) (Model, error)
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Model, error)
}

type Service struct{ repository repository }

func NewService(repository repository) *Service { return &Service{repository: repository} }

func NewTransactionNo() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate transaction number: %w", err)
	}
	return "PT" + hex.EncodeToString(bytes), nil
}
