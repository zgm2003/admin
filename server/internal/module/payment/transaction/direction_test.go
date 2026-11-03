package transaction

import (
	"context"
	"math"
	"net/url"
	"testing"

	"admin/server/internal/shared/i18n"
)

func TestTransactionDirectionAndPersistedBalance(t *testing.T) {
	for _, fixture := range []struct {
		direction             Direction
		source                SourceType
		before, amount, after int64
	}{
		{DirectionCredit, SourcePayment, -10, 4, -6},
		{DirectionCredit, SourceRedeemCode, -10, 4, -6},
		{DirectionDebit, SourceAI, 2, 4, -2},
		{DirectionDebit, SourceAI, math.MinInt64 + 1, 1, math.MinInt64},
	} {
		row := Model{ID: 1, UserID: 2, PlatformID: 3, Direction: fixture.direction, AmountUnits: fixture.amount, BalanceBeforeUnits: fixture.before, BalanceAfterUnits: fixture.after}
		switch fixture.source {
		case SourcePayment:
			row.OrderID = ptrInt64(4)
		case SourceRedeemCode:
			row.RedeemCodeID = ptrInt64(5)
		case SourceAI:
			row.AIUsageChargeID = ptrInt64(6)
		}
		got, err := value(context.Background(), row)
		if err != nil || got.Direction != fixture.direction || got.SourceType != fixture.source {
			t.Fatalf("row=%+v got=%+v err=%v", row, got, err)
		}
	}
}

func TestTransactionRejectsInvalidSourceDirectionAndArithmetic(t *testing.T) {
	valid := Model{ID: 1, UserID: 2, PlatformID: 3, Direction: DirectionDebit, AmountUnits: 4, BalanceBeforeUnits: 2, BalanceAfterUnits: -2, AIUsageChargeID: ptrInt64(6)}
	for name, change := range map[string]func(*Model){
		"missing source":     func(m *Model) { m.AIUsageChargeID = nil },
		"multiple sources":   func(m *Model) { m.OrderID = ptrInt64(1) },
		"invalid source id":  func(m *Model) { m.AIUsageChargeID = ptrInt64(0) },
		"credit AI":          func(m *Model) { m.Direction = DirectionCredit },
		"debit order":        func(m *Model) { m.AIUsageChargeID = nil; m.OrderID = ptrInt64(1) },
		"debit redeem":       func(m *Model) { m.AIUsageChargeID = nil; m.RedeemCodeID = ptrInt64(1) },
		"invalid direction":  func(m *Model) { m.Direction = 3 },
		"nonpositive amount": func(m *Model) { m.AmountUnits = 0 },
		"wrong before":       func(m *Model) { m.BalanceBeforeUnits = 3 },
		"debit underflow":    func(m *Model) { m.BalanceBeforeUnits = math.MinInt64 },
		"credit overflow": func(m *Model) {
			m.Direction = DirectionCredit
			m.AIUsageChargeID = nil
			m.OrderID = ptrInt64(1)
			m.BalanceBeforeUnits = math.MaxInt64
		},
	} {
		t.Run(name, func(t *testing.T) {
			row := valid
			change(&row)
			if _, err := value(context.Background(), row); err == nil {
				t.Fatalf("invalid row accepted: %+v", row)
			}
		})
	}
}

func TestTransactionDirectionAndSourceFilters(t *testing.T) {
	values := url.Values{"page": {"1"}, "pageSize": {"20"}, "direction": {"2"}, "sourceType": {"3"}}
	for _, personal := range []bool{false, true} {
		var query ListQuery
		var err error
		if personal {
			query, err = parseListQuery(values, 1, 2, true)
		} else {
			query, err = parseAdminListQuery(values)
		}
		if err != nil || query.Direction != DirectionDebit || query.SourceType != SourceAI {
			t.Fatalf("query=%+v err=%v", query, err)
		}
	}
	for _, suffix := range []string{"direction=0", "direction=3", "direction=", "direction=1&direction=2", "sourceType=0", "sourceType=4", "sourceType=-1", "sourceType=1.0"} {
		values, _ := url.ParseQuery("page=1&pageSize=20&" + suffix)
		if _, err := parseAdminListQuery(values); err == nil {
			t.Fatalf("invalid filters accepted: %s", suffix)
		}
	}
}

func TestTransactionStaticOptionsAreLocalizedWithoutRepository(t *testing.T) {
	for _, fixture := range []struct {
		locale            i18n.Locale
		direction, source string
	}{{i18n.ZhCN, "消费", "AI 消费"}, {i18n.EnUS, "Debit", "AI usage"}} {
		ctx := i18n.WithLocale(context.Background(), fixture.locale)
		got, err := NewService(nil).Options(ctx)
		if err != nil || len(got.Directions) != 2 || len(got.SourceTypes) != 3 || got.Directions[1].Label != fixture.direction || got.SourceTypes[2].Label != fixture.source {
			t.Fatalf("options=%+v err=%v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewService(nil).Options(ctx); err == nil {
		t.Fatal("cancelled options succeeded")
	}
}
