package setting

import (
	"context"
	"errors"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestRechargeMinimumSettingValidation(t *testing.T) {
	const key = "payment.recharge.minimum_amount"
	for _, tc := range []struct {
		value  string
		typeID int
		valid  bool
	}{
		{"1", ValueTypeNumber, true},
		{"10", ValueTypeNumber, true},
		{"92233720368", ValueTypeNumber, true},
		{"0", ValueTypeNumber, false},
		{"-1", ValueTypeNumber, false},
		{"92233720369", ValueTypeNumber, false},
		{"10.5", ValueTypeNumber, false},
		{"+10", ValueTypeNumber, false},
		{"000000000010", ValueTypeNumber, false},
		{"", ValueTypeNumber, false},
		{"999999999999999999999999", ValueTypeNumber, false},
		{"10", ValueTypeString, false},
	} {
		if err := validateInput(key, tc.value, tc.typeID, ""); (err == nil) != tc.valid {
			t.Errorf("value=%s type=%d valid=%v err=%v", tc.value, tc.typeID, tc.valid, err)
		}
	}
	if !isRequiredSetting(key) {
		t.Error("minimum recharge setting may not be disabled/deleted")
	}
	p := presentationFor(Record{Key: key, ValueType: ValueTypeNumber, IsBuiltin: yesno.Yes})
	if p.Editor != "number" || !p.ValueTypeLocked || p.Minimum == nil || *p.Minimum != 1 || p.Maximum == nil || *p.Maximum != 92233720368 || p.AllowEmpty || p.Actions.Status || p.Actions.Delete {
		t.Fatalf("presentation=%+v", p)
	}
}

func TestRechargeMinimumSettingRejectsInvalidMutationBeforeRepositoryAccess(t *testing.T) {
	for _, operation := range []string{"create", "update", "disable", "delete"} {
		t.Run(operation, func(t *testing.T) {
			repository := &fakeRepository{findErr: errors.New("repository must not be accessed")}
			service := NewService(repository)
			const key = " payment.recharge.minimum_amount "
			var err error
			switch operation {
			case "create":
				_, err = service.Create(context.Background(), CreateInput{Key: key, Value: "0", ValueType: ValueTypeNumber})
			case "update":
				err = service.Update(context.Background(), key, UpdateInput{Value: "0", ValueType: ValueTypeNumber})
			case "disable":
				err = service.UpdateStatus(context.Background(), key, yesno.No)
			case "delete":
				err = service.Delete(context.Background(), key)
			}
			var public *apperror.Error
			if !errors.As(err, &public) {
				t.Fatalf("error=%v, expected public validation error", err)
			}
			wantCode := apperror.CodeInvalidRequest
			if operation == "delete" {
				wantCode = apperror.CodeConflict
			}
			if public.Code != wantCode {
				t.Fatalf("code=%d want=%d", public.Code, wantCode)
			}
			if repository.findCalls() != 0 || repository.mutations() != 0 {
				t.Fatalf("invalid %s touched repository: finds=%d mutations=%d", operation, repository.findCalls(), repository.mutations())
			}
		})
	}
}
