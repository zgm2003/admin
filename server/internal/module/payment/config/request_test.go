package config

import (
	"admin/server/internal/module/payment"
	"encoding/json"
	"strings"
	"testing"
)

func TestNotifyURLTemplateBindsCoordinates(t *testing.T) {
	template := "https://pay.example.test/api/v1/payment/callback/alipay/{configId}/{version}"
	got, err := resolveNotifyURL(template, 42, 3)
	if err != nil || got != "https://pay.example.test/api/v1/payment/callback/alipay/42/3" {
		t.Fatalf("%q %v", got, err)
	}
	for _, url := range []string{"", "http://pay.example.test/api/v1/payment/callback/alipay/{configId}/{version}", "https://user:pass@pay.example.test/api/v1/payment/callback/alipay/{configId}/{version}", strings.Replace(template, "alipay", "wechat", 1), template + "?token=secret", strings.Replace(template, "{version}", "1", 1)} {
		if _, err := resolveNotifyURL(url, 42, 1); err == nil {
			t.Fatalf("accepted %q", url)
		}
	}
}
func TestVersionRequestRequiresExplicitRetainFields(t *testing.T) {
	var r versionRequest
	if err := json.Unmarshal([]byte(`{"privateKey":null,"appCertPEM":null,"alipayCertPEM":null,"rootCertPEM":null,"notifyUrl":"https://pay.example.test/api/v1/payment/callback/alipay/{configId}/{version}","payMethods":[2,1]}`), &r); err != nil {
		t.Fatal(err)
	}
	v, err := r.input()
	if err != nil || v.PrivateKey != nil || v.PayMethods[0] != payment.PayMethodWeb {
		t.Fatalf("%+v %v", v, err)
	}
	for _, raw := range []string{`{}`, `{"privateKey":"","appCertPEM":null,"alipayCertPEM":null,"rootCertPEM":null,"notifyUrl":"x","payMethods":[1]}`, `{"privateKey":null,"appCertPEM":null,"alipayCertPEM":null,"notifyUrl":"x","payMethods":[1]}`} {
		var bad versionRequest
		if err := json.Unmarshal([]byte(raw), &bad); err == nil {
			if _, err := bad.input(); err == nil {
				t.Fatal("partial replacement accepted")
			}
		}
	}
}
