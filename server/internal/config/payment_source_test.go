package config

import "testing"

func TestRuntimeConfigDoesNotRequirePaymentEnvironmentOrReturnOrigins(t *testing.T) {
	for _, configured := range []bool{false, true} {
		values := validAPIValues()
		delete(values, "PAYMENT_ENVIRONMENT")
		delete(values, "PAYMENT_RETURN_ORIGINS")
		if configured {
			// Obsolete payment variables must not override database configuration.
			values["PAYMENT_ENVIRONMENT"] = "not-an-environment"
			values["PAYMENT_RETURN_ORIGINS"] = "not-an-origin"
		}
		if _, err := LoadAPI(lookup(values)); err != nil {
			t.Errorf("API configured=%v: %v", configured, err)
		}
		if _, err := LoadWorker(lookup(values)); err != nil {
			t.Errorf("Worker configured=%v: %v", configured, err)
		}
	}
}
