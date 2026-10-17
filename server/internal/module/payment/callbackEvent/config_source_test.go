package callbackevent

import (
	"context"
	"testing"

	"admin/server/internal/module/payment"
)

func TestCallbackUsesHistoricalCredentialEnvironmentWithoutDeploymentSetting(t *testing.T) {
	for _, environment := range []payment.Environment{payment.EnvironmentSandbox, payment.EnvironmentProduction} {
		f := newEventFixture(t)
		f.credentials.Environment = environment
		service := NewService(f.repo)
		service.runtime = eventRuntimeStub{credentials: f.credentials}
		service.SetConfirmer(f.confirmer)
		service.now = f.service.now
		if _, err := service.Receive(context.Background(), 3, 4, f.signed(t, nil)); err != nil {
			t.Fatalf("environment=%d callback err=%v", environment, err)
		}
		if f.confirmer.calls != 1 || f.confirmer.last.Environment != environment {
			t.Fatalf("environment=%d confirmed=%+v calls=%d", environment, f.confirmer.last, f.confirmer.calls)
		}
	}
}
