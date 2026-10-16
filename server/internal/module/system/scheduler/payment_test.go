package scheduler

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPaymentReconcileDefinitionIsBuiltinAndRejectsParameters(t *testing.T) {
	definition := PaymentReconcileDefinition(func(context.Context, ExecutionContext) error { return nil })
	if definition.Type != "payment.order.reconcile" || definition.BuiltinKey != "payment.order.reconcile" || definition.Timeout <= 0 {
		t.Fatalf("definition = %+v", definition)
	}
	if err := definition.ValidateParams(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("empty params rejected: %v", err)
	}
	if err := definition.ValidateParams(json.RawMessage(`{"limit":1}`)); err == nil {
		t.Fatal("payment reconcile accepted unsupported params")
	}
}
