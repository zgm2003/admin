package rechargepackage

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/money"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
}

type Presentation struct {
	Status string `json:"status"`
}

type SafeValue struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	Amount       string       `json:"amount"`
	Badge        *string      `json:"badge"`
	SortOrder    int32        `json:"sortOrder"`
	Status    yesno.Value  `json:"status"`
	Actions      Actions      `json:"actions"`
	Presentation Presentation `json:"presentation"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

type AvailablePackage struct {
	ID          int64
	Name        string
	AmountCents int64
	Badge       *string
	SortOrder   int32
}

func safeValue(ctx context.Context, row Model) (SafeValue, error) {
	if row.ID < 1 || validateName(row.Name) != nil || validateAmountCents(row.AmountCents) != nil || validateFlags(row.Status) != nil {
		return SafeValue{}, fmt.Errorf("stored recharge package is invalid")
	}
	badge, err := normalizeBadge(row.Badge)
	if err != nil {
		return SafeValue{}, err
	}
	amount, err := money.FormatRMBCents(row.AmountCents)
	if err != nil {
		return SafeValue{}, err
	}
	status := option.New(ctx, row.Status, "停用", "Disabled").Label
	if row.Status == yesno.Yes {
		status = option.New(ctx, row.Status, "启用", "Enabled").Label
	}
	return SafeValue{
		ID: row.ID, Name: row.Name, Amount: amount, Badge: badge,
		SortOrder: row.SortOrder, Status: row.Status,
		Actions:      Actions{Update: true, Status: true, Delete: true},
		Presentation: Presentation{Status: status}, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func availablePackage(row CatalogItem) AvailablePackage {
	return AvailablePackage{ID: row.ID, Name: row.Name, AmountCents: row.AmountCents, Badge: copyString(row.Badge), SortOrder: row.SortOrder}
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
