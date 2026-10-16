package alipay

import (
	"admin/server/internal/module/payment"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-pay/gopay/pkg/xhttp"
)

func adapterCredentials(t *testing.T) (Credentials, *rsa.PrivateKey) {
	t.Helper()
	now := time.Now().UTC()
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	appKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	aliKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rootCert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fixture Root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, SignatureAlgorithm: x509.SHA256WithRSA}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootCert, rootCert, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	makeLeaf := func(serial int64, name string, key *rsa.PrivateKey) string {
		t.Helper()
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, SignatureAlgorithm: x509.SHA256WithRSA}
		der, createErr := x509.CreateCertificate(rand.Reader, leaf, rootCert, &key.PublicKey, rootKey)
		if createErr != nil {
			t.Fatal(createErr)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	return Credentials{
		AppID: "20260001", SellerID: "20880001", Environment: payment.EnvironmentSandbox,
		NotifyURL:  "https://pay.example.test/api/v1/payment/callback/alipay/1/1",
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(appKey)})),
		AppCertPEM: makeLeaf(2, "20260001", appKey), AlipayCertPEM: makeLeaf(3, "Alipay Fixture", aliKey),
		RootCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})),
	}, aliKey
}

func TestCheckoutMapsExactAmountMethodAndReturnURL(t *testing.T) {
	credentials, _ := adapterCredentials(t)
	for _, test := range []struct {
		method      payment.PayMethod
		wantMethod  string
		wantProduct string
	}{
		{payment.PayMethodWeb, "alipay.trade.page.pay", "FAST_INSTANT_TRADE_PAY"},
		{payment.PayMethodH5, "alipay.trade.wap.pay", "QUICK_WAP_WAY"},
	} {
		t.Run(test.wantMethod, func(t *testing.T) {
			checkout, err := Checkout(context.Background(), credentials, CheckoutInput{
				OrderNo: "ORDER-001", Subject: "充值 12.34", AmountCents: 1234,
				Method: test.method, ReturnURL: "https://app.example.test/payment/return?order=ORDER-001",
			})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(checkout)
			if err != nil {
				t.Fatal(err)
			}
			query := parsed.Query()
			var body map[string]string
			if err := json.Unmarshal([]byte(query.Get("biz_content")), &body); err != nil {
				t.Fatal(err)
			}
			if query.Get("method") != test.wantMethod || body["product_code"] != test.wantProduct || body["out_trade_no"] != "ORDER-001" || body["total_amount"] != "12.34" || body["subject"] != "充值 12.34" || query.Get("return_url") != "https://app.example.test/payment/return?order=ORDER-001" {
				t.Fatalf("checkout parameters do not match input: method=%q return=%q body=%v", query.Get("method"), query.Get("return_url"), body)
			}
			if query.Get("notify_url") != credentials.NotifyURL {
				t.Fatalf("notify URL = %q", query.Get("notify_url"))
			}
		})
	}
}

func TestCheckoutRejectsInvalidInputWithoutDefaultingMethod(t *testing.T) {
	credentials, _ := adapterCredentials(t)
	for _, input := range []CheckoutInput{
		{OrderNo: "ORDER", Subject: "x", AmountCents: 1, Method: 0, ReturnURL: "https://app.example.test/return"},
		{OrderNo: "ORDER", Subject: "x", AmountCents: 1, Method: 3, ReturnURL: "https://app.example.test/return"},
		{OrderNo: "ORDER", Subject: "x", AmountCents: 0, Method: payment.PayMethodWeb, ReturnURL: "https://app.example.test/return"},
		{OrderNo: "ORDER", Subject: "x", AmountCents: -1, Method: payment.PayMethodWeb, ReturnURL: "https://app.example.test/return"},
	} {
		if _, err := Checkout(context.Background(), credentials, input); err == nil {
			t.Fatalf("invalid input accepted: %+v", input)
		}
	}
}

func TestVerifyNotifyVerifiesRSA2AndParsesTrustedPaymentFacts(t *testing.T) {
	credentials, alipayKey := adapterCredentials(t)
	values := signedNotifyValues(t, alipayKey, url.Values{
		"notify_id": {"NOTIFY-1"}, "app_id": {credentials.AppID}, "seller_id": {credentials.SellerID},
		"out_trade_no": {"ORDER-001"}, "trade_no": {"TRADE-001"}, "trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"}, "gmt_payment": {"2026-10-15 12:34:56"}, "sign_type": {"RSA2"},
	})
	notify, err := VerifyNotify(context.Background(), credentials, values)
	if err != nil {
		t.Fatal(err)
	}
	wantPaidAt := time.Date(2026, 10, 15, 4, 34, 56, 0, time.UTC)
	if notify.NotifyID != "NOTIFY-1" || notify.OrderNo != "ORDER-001" || notify.TradeNo != "TRADE-001" || notify.AppID != credentials.AppID || notify.SellerID == nil || *notify.SellerID != credentials.SellerID || notify.AmountCents != 1234 || notify.Status != "TRADE_SUCCESS" || notify.PaidAt == nil || !notify.PaidAt.Equal(wantPaidAt) {
		t.Fatalf("unexpected notify facts: %+v", notify)
	}
}

func TestVerifyNotifyRejectsTamperingDuplicatesAndMalformedFacts(t *testing.T) {
	credentials, key := adapterCredentials(t)
	base := url.Values{
		"notify_id": {"N1"}, "app_id": {credentials.AppID}, "seller_id": {credentials.SellerID},
		"out_trade_no": {"ORDER-001"}, "trade_no": {"TRADE-001"}, "trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"}, "sign_type": {"RSA2"},
	}
	tests := map[string]func(url.Values){
		"tampered":          func(v url.Values) { v.Set("total_amount", "999.00") },
		"duplicate field":   func(v url.Values) { v.Add("total_amount", "12.34") },
		"non RSA2":          func(v url.Values) { v.Set("sign_type", "RSA") },
		"malformed amount":  func(v url.Values) { v.Set("total_amount", "12.345") },
		"missing order":     func(v url.Values) { v.Del("out_trade_no") },
		"identity mismatch": func(v url.Values) { v.Set("app_id", "99999999") },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			values := signedNotifyValues(t, key, base)
			change(values)
			if _, err := VerifyNotify(context.Background(), credentials, values); err == nil {
				t.Fatal("invalid notification accepted")
			}
		})
	}
	tooLarge := signedNotifyValues(t, key, base)
	tooLarge.Set("unknown", strings.Repeat("x", maxNotifyBytes))
	if _, err := VerifyNotify(context.Background(), credentials, tooLarge); err == nil {
		t.Fatal("oversized notification accepted")
	}
}

func TestVerifyNotifyRejectsSignedMalformedIdentityAndBounds(t *testing.T) {
	credentials, key := adapterCredentials(t)
	base := url.Values{
		"notify_id": {"N1"}, "app_id": {credentials.AppID}, "seller_id": {credentials.SellerID},
		"out_trade_no": {"ORDER-001"}, "trade_no": {"TRADE-001"}, "trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"},
	}
	for name, change := range map[string]func(url.Values){
		"missing seller":    func(v url.Values) { v.Del("seller_id") },
		"padded seller":     func(v url.Values) { v.Set("seller_id", " "+credentials.SellerID) },
		"oversize order":    func(v url.Values) { v.Set("out_trade_no", strings.Repeat("A", 33)) },
		"oversize trade":    func(v url.Values) { v.Set("trade_no", strings.Repeat("A", 65)) },
		"oversize notify":   func(v url.Values) { v.Set("notify_id", strings.Repeat("A", 129)) },
		"oversize identity": func(v url.Values) { v.Set("app_id", strings.Repeat("A", 65)) },
		"control status":    func(v url.Values) { v.Set("trade_status", "TRADE_SUCCESS\n") },
		"too many fields": func(v url.Values) {
			for i := 0; i < 65; i++ {
				v.Set(fmt.Sprintf("extra_%d", i), "x")
			}
		},
		"uncreditable amount": func(v url.Values) { v.Set("total_amount", "92233720368.55") },
	} {
		t.Run(name, func(t *testing.T) {
			values := make(url.Values, len(base))
			for k, v := range base {
				values[k] = append([]string(nil), v...)
			}
			change(values)
			if _, err := VerifyNotify(context.Background(), credentials, signedNotifyValues(t, key, values)); err == nil {
				t.Fatal("signed malformed facts accepted")
			}
		})
	}
	if _, err := VerifyNotify(nil, credentials, signedNotifyValues(t, key, base)); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestVerifyNotifyPreservesVerifiedIdentityMismatchAndOptionalNotifyID(t *testing.T) {
	credentials, key := adapterCredentials(t)
	values := signedNotifyValues(t, key, url.Values{
		"app_id": {"OTHER-APP"}, "seller_id": {"OTHER-SELLER"}, "out_trade_no": {"ORDER-001"},
		"trade_no": {"TRADE-001"}, "trade_status": {"TRADE_SUCCESS"}, "total_amount": {"12.34"},
	})
	notify, err := VerifyNotify(context.Background(), credentials, values)
	if err != nil || notify.NotifyID != "" || notify.AppID != "OTHER-APP" || notify.SellerID == nil || *notify.SellerID != "OTHER-SELLER" {
		t.Fatalf("verified mismatched facts must reach audit/settlement checks: notify=%+v err=%v", notify, err)
	}
}

func signedNotifyValues(t *testing.T, key *rsa.PrivateKey, values url.Values) url.Values {
	t.Helper()
	copyValues := make(url.Values, len(values))
	for name, entries := range values {
		copyValues[name] = append([]string(nil), entries...)
	}
	copyValues.Del("sign")
	copyValues.Del("sign_type")
	data := make([]string, 0, len(copyValues))
	for name, entries := range copyValues {
		if len(entries) == 1 {
			data = append(data, name+"="+entries[0])
		}
	}
	// Alipay signs keys in lexical order.
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if data[j][:strings.Index(data[j], "=")] < data[i][:strings.Index(data[i], "=")] {
				data[i], data[j] = data[j], data[i]
			}
		}
	}
	signature := signRSA2(t, key, strings.Join(data, "&"))
	copyValues.Set("sign", signature)
	copyValues.Set("sign_type", "RSA2")
	return copyValues
}

func signRSA2(t *testing.T, key *rsa.PrivateKey, message string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestQueryAndCloseUseExpectedTradeIdentifiers(t *testing.T) {
	credentials, alipayKey := adapterCredentials(t)
	for _, operation := range []string{"query", "close"} {
		t.Run(operation, func(t *testing.T) {
			requestSeen := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestSeen = true
				if err := r.ParseForm(); err != nil {
					t.Errorf("parse gateway request: %v", err)
					return
				}
				var biz map[string]string
				if err := json.Unmarshal([]byte(r.Form.Get("biz_content")), &biz); err != nil {
					t.Errorf("parse biz content: %v", err)
					return
				}
				wantMethod := "alipay.trade." + operation
				if r.Form.Get("method") != wantMethod || biz["out_trade_no"] != "ORDER-001" {
					t.Errorf("request method/id = %q/%q", r.Form.Get("method"), biz["out_trade_no"])
				}
				responseKey := "alipay_trade_" + operation + "_response"
				response := map[string]string{"code": "10000", "out_trade_no": "ORDER-001", "trade_no": "TRADE-001"}
				if operation == "query" {
					response["trade_status"] = "TRADE_SUCCESS"
					response["total_amount"] = "12.34"
					response["send_pay_date"] = "2026-10-15 12:34:56"
				}
				inner, marshalErr := json.Marshal(response)
				if marshalErr != nil {
					t.Error(marshalErr)
					return
				}
				appClient, clientErr := NewClient(context.Background(), credentials)
				if clientErr != nil {
					t.Error(clientErr)
					return
				}
				certSN := appClient.AliPayPublicCertSN
				sign := signRSA2(t, alipayKey, string(inner))
				payload := fmt.Sprintf(`{"%s":%s,"alipay_cert_sn":%q,"sign":%q}`, responseKey, inner, certSN, sign)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, payload)
			}))
			defer server.Close()

			client, err := NewClient(context.Background(), credentials)
			if err != nil {
				t.Fatal(err)
			}
			testURL, _ := url.Parse(server.URL)
			client.GetHttpClient().SetTransport(transportFunc(func(req *http.Request) (*http.Response, error) {
				clone := req.Clone(req.Context())
				clone.URL.Scheme, clone.URL.Host = testURL.Scheme, testURL.Host
				clone.Host = testURL.Host
				return http.DefaultTransport.RoundTrip(clone)
			}))
			var trade Trade
			if operation == "query" {
				trade, err = queryWithClient(context.Background(), credentials, client, "ORDER-001")
			} else {
				trade, err = closeWithClient(context.Background(), credentials, client, "ORDER-001")
			}
			if err != nil {
				t.Fatal(err)
			}
			if !requestSeen || trade.OrderNo != "ORDER-001" || trade.TradeNo != "TRADE-001" || trade.AppID != credentials.AppID {
				t.Fatalf("request/result mismatch: seen=%v trade=%+v", requestSeen, trade)
			}
			if operation == "query" && (trade.AmountCents != 1234 || trade.Status != "TRADE_SUCCESS" || trade.PaidAt == nil) {
				t.Fatalf("query facts mismatch: %+v", trade)
			}
		})
	}
}

func TestQueryRejectsMismatchedOrderAndMalformedAmount(t *testing.T) {
	credentials, alipayKey := adapterCredentials(t)
	for _, response := range []map[string]string{
		{"code": "10000", "out_trade_no": "OTHER", "trade_no": "TRADE-001", "trade_status": "TRADE_SUCCESS", "total_amount": "12.34"},
		{"code": "10000", "out_trade_no": "ORDER-001", "trade_no": "TRADE-001", "trade_status": "TRADE_SUCCESS", "total_amount": "12.345"},
	} {
		client, err := NewClient(context.Background(), credentials)
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := json.Marshal(response)
		payload := signedQueryResponse(t, credentials, alipayKey, inner)
		client.GetHttpClient().SetTransport(transportFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload)), Request: req}, nil
		}))
		if _, err := queryWithClient(context.Background(), credentials, client, "ORDER-001"); err == nil {
			t.Fatalf("invalid query response accepted: %s", inner)
		}
	}
}

func signedQueryResponse(t *testing.T, credentials Credentials, key *rsa.PrivateKey, inner []byte) string {
	t.Helper()
	client, err := NewClient(context.Background(), credentials)
	if err != nil {
		t.Fatal(err)
	}
	sign := signRSA2(t, key, string(inner))
	return fmt.Sprintf(`{"alipay_trade_query_response":%s,"alipay_cert_sn":%q,"sign":%q}`, inner, client.AliPayPublicCertSN, sign)
}

var _ = xhttp.NewClient
