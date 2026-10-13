package payment

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPaymentNumericContract(t *testing.T) {
	type input struct {
		Provider    Provider    `json:"provider"`
		Environment Environment `json:"environment"`
		PayMethods  []PayMethod `json:"payMethods"`
	}
	var v input
	if err := json.Unmarshal([]byte(`{"provider":1,"environment":2,"payMethods":[2,1]}`), &v); err != nil {
		t.Fatal(err)
	}
	methods, err := NormalizePayMethods(v.PayMethods)
	if err != nil || v.Provider != ProviderAlipay || v.Environment != EnvironmentProduction || !reflect.DeepEqual(methods, []PayMethod{1, 2}) {
		t.Fatalf("contract: %+v %v %v", v, methods, err)
	}
	for _, raw := range []string{`{"provider":"1"}`, `{"environment":"sandbox"}`, `{"payMethods":["web"]}`, `{"provider":1.5}`, `{"environment":65538}`} {
		if err := json.Unmarshal([]byte(raw), &input{}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if ProviderWechat.Supported() || !ProviderAlipay.Supported() {
		t.Fatal("unimplemented provider advertised")
	}
	for _, raw := range []string{"", "0", "3", "+1", " 1", "01", "sandbox"} {
		if _, err := ParseEnvironment(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"1", "2"} {
		if _, err := ParseEnvironment(raw); err != nil {
			t.Fatal(err)
		}
	}
}
func TestPayMethodsRejectInvalidSets(t *testing.T) {
	for _, v := range [][]PayMethod{nil, {}, {1, 1}, {0}, {3}, {-1}, {1, 2, 1}} {
		if _, err := NormalizePayMethods(v); err == nil {
			t.Fatalf("accepted %v", v)
		}
	}
	v := []PayMethod{2, 1}
	got, err := NormalizePayMethods(v)
	if err != nil || got[0] != 1 || v[0] != 2 {
		t.Fatal("normalization mutated caller")
	}
}
