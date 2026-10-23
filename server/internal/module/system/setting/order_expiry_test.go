package setting

import (
	"context"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestOrderExpirySettingValidationAndPresentation(t *testing.T) {
	const key = "payment.order.expiry_minutes"
	for _, value := range []string{"1", "30", "1440"} {
		if err := validateInput(key, value, ValueTypeNumber, ""); err != nil {
			t.Fatalf("valid minutes %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "0", "-1", "1441", "1.5", "+30", " 30 ", "99999999999999999999"} {
		if err := validateInput(key, value, ValueTypeNumber, ""); err == nil {
			t.Errorf("invalid minutes accepted: %q", value)
		}
	}
	if err := validateInput(key, "30", ValueTypeString, ""); err == nil {
		t.Error("wrong value type accepted")
	}
	p := itemResponse(Record{Key: key, ValueType: ValueTypeNumber, IsBuiltin: yesno.Yes}).Presentation
	if p.Editor != "number" || !p.ValueTypeLocked || p.AllowEmpty || p.Minimum == nil || *p.Minimum != 1 || p.Maximum == nil || *p.Maximum != 1440 || p.Actions.Status || p.Actions.Delete {
		t.Errorf("incorrect expiry editor constraints: %+v", p)
	}
	repo := &fakeRepository{}
	s := NewService(repo)
	if err := s.UpdateStatus(context.Background(), key, yesno.No); err == nil {
		t.Error("required expiry can be disabled")
	}
	if err := s.Delete(context.Background(), key); err == nil {
		t.Error("required expiry can be deleted")
	}
	if repo.findCalls() != 0 || repo.mutations() != 0 {
		t.Error("protected operation accessed repository")
	}
}
