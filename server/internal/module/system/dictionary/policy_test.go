package dictionary

import (
	"errors"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestFixedGenderPolicyRejectsNewValuesAndDisabling(t *testing.T) {
	parent := Dictionary{Code: fixedGenderDictionaryCode, IsBuiltin: yesno.Yes}
	if err := validateFixedValueCreate(parent, "3"); !errors.Is(err, ErrConflict) {
		t.Fatalf("new gender value error = %v, want conflict", err)
	}
	if err := validateFixedValueCreate(parent, "1"); err != nil {
		t.Fatalf("fixed gender value was rejected: %v", err)
	}
	if err := validateFixedDictionaryStatus(parent, yesno.No); !errors.Is(err, ErrConflict) {
		t.Fatalf("disable gender dictionary error = %v, want conflict", err)
	}
	item := Item{Value: "1", IsEnabled: yesno.Yes}
	if err := validateFixedValueStatus(parent, item, yesno.No); !errors.Is(err, ErrConflict) {
		t.Fatalf("disable gender value error = %v, want conflict", err)
	}
	if err := validateFixedValueDelete(parent, item); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete gender value error = %v, want conflict", err)
	}
}

func TestFixedGenderPolicyLeavesCustomDictionariesExtensible(t *testing.T) {
	parent := Dictionary{Code: "user.level", IsBuiltin: yesno.No}
	item := Item{Value: "custom", IsEnabled: yesno.Yes}
	if err := validateFixedValueCreate(parent, "new"); err != nil {
		t.Fatalf("custom value was rejected: %v", err)
	}
	if err := validateFixedValueStatus(parent, item, yesno.No); err != nil {
		t.Fatalf("custom status change was rejected: %v", err)
	}
	if err := validateFixedValueDelete(parent, item); err != nil {
		t.Fatalf("custom value deletion was rejected: %v", err)
	}
}
