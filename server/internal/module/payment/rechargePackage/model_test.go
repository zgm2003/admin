package rechargepackage

import (
	"reflect"
	"testing"
)

func TestModelMapsThePaymentRechargePackageContract(t *testing.T) {
	modelType := reflect.TypeOf(Model{})
	if (Model{}).TableName() != "payment_recharge_package" {
		t.Fatalf("table name=%q", (Model{}).TableName())
	}
	for _, field := range []string{"Code", "Name", "AmountCents", "Badge", "SortOrder", "IsEnabled", "CreatedAt", "UpdatedAt", "DeletedAt"} {
		if _, ok := modelType.FieldByName(field); !ok {
			t.Fatalf("missing model field %s", field)
		}
	}
}
