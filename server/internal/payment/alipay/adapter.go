package alipay

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/money"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-pay/gopay"
	sdk "github.com/go-pay/gopay/alipay"
)

const maxNotifyBytes = 64 * 1024

// CheckoutInput contains only facts that are safe for the order service to
// pass to the provider. AmountCents is an integer amount in RMB cents.
type CheckoutInput struct {
	OrderNo     string
	Subject     string
	AmountCents int64
	Method      payment.PayMethod
	ReturnURL   string
}

// Trade is the provider's verified representation of an order. AppID is the
// configured application identity; the gateway does not echo it in all APIs.
type Trade struct {
	OrderNo     string
	TradeNo     string
	AppID       string
	SellerID    *string
	AmountCents int64
	Status      string
	PaidAt      *time.Time
}

// Notify is an RSA2-verified asynchronous payment notification.
type Notify struct {
	NotifyID    string
	OrderNo     string
	TradeNo     string
	AppID       string
	SellerID    *string
	AmountCents int64
	Status      string
	PaidAt      *time.Time
}

var (
	ErrCheckoutInvalid      = errors.New("alipay checkout input invalid")
	ErrUnsupportedPayMethod = errors.New("alipay payment method unsupported")
	ErrCheckoutGateway      = errors.New("alipay checkout unavailable")
	ErrQueryGateway         = errors.New("alipay query unavailable")
	ErrCloseGateway         = errors.New("alipay close unavailable")
	ErrTradeInvalid         = errors.New("alipay trade response invalid")
	ErrNotifyInvalid        = errors.New("alipay notification invalid")
	ErrNotifySignature      = errors.New("alipay notification signature invalid")
)

// Checkout creates the signed redirect URL for a PC or H5 checkout. It does
// not call a remote endpoint: GoPay signs the URL locally.
func Checkout(ctx context.Context, credentials Credentials, input CheckoutInput) (string, error) {
	if err := validateCheckoutInput(ctx, input); err != nil {
		return "", err
	}
	amount, err := money.FormatRMBCents(input.AmountCents)
	if err != nil {
		return "", ErrCheckoutInvalid
	}
	client, err := NewClient(ctx, credentials)
	if err != nil {
		return "", err
	}
	body := gopay.BodyMap{
		"out_trade_no": input.OrderNo,
		"total_amount": amount,
		"subject":      input.Subject,
		"return_url":   input.ReturnURL,
	}
	var checkout string
	switch input.Method {
	case payment.PayMethodWeb:
		checkout, err = client.TradePagePay(ctx, body)
	case payment.PayMethodH5:
		checkout, err = client.TradeWapPay(ctx, body)
	default:
		return "", ErrUnsupportedPayMethod
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrCheckoutGateway
	}
	if checkout == "" || checkout == gopay.NULL {
		return "", ErrCheckoutGateway
	}
	return checkout, nil
}

// Query asks Alipay for the current state of an order and verifies the signed
// gateway response through GoPay's certificate mode.
func Query(ctx context.Context, credentials Credentials, orderNo string) (Trade, error) {
	if err := validateOrderNo(ctx, orderNo); err != nil {
		return Trade{}, err
	}
	client, err := NewClient(ctx, credentials)
	if err != nil {
		return Trade{}, err
	}
	return queryWithClient(ctx, credentials, client, orderNo)
}

func queryWithClient(ctx context.Context, credentials Credentials, client *sdk.Client, orderNo string) (Trade, error) {
	if err := validateOrderNo(ctx, orderNo); err != nil {
		return Trade{}, err
	}
	response, err := client.TradeQuery(ctx, gopay.BodyMap{"out_trade_no": orderNo})
	if err != nil {
		if ctx.Err() != nil {
			return Trade{}, ctx.Err()
		}
		return Trade{}, ErrQueryGateway
	}
	if response == nil || response.Response == nil {
		return Trade{}, ErrTradeInvalid
	}
	value := response.Response
	if value.OutTradeNo != orderNo || value.TradeNo == "" || value.TradeStatus == "" {
		return Trade{}, ErrTradeInvalid
	}
	amount, err := money.ParseRMBCents(value.TotalAmount)
	if err != nil || amount <= 0 {
		return Trade{}, ErrTradeInvalid
	}
	var paidAt *time.Time
	if value.SendPayDate != "" {
		paidAt, err = parseProviderTime(value.SendPayDate)
		if err != nil {
			return Trade{}, ErrTradeInvalid
		}
	}
	return Trade{OrderNo: value.OutTradeNo, TradeNo: value.TradeNo, AppID: credentials.AppID, AmountCents: amount, Status: value.TradeStatus, PaidAt: paidAt}, nil
}

// Close asks Alipay to close an unpaid order. The provider close response has
// no amount/status, so those fields remain zero/empty in the returned Trade.
func Close(ctx context.Context, credentials Credentials, orderNo string) (Trade, error) {
	if err := validateOrderNo(ctx, orderNo); err != nil {
		return Trade{}, err
	}
	client, err := NewClient(ctx, credentials)
	if err != nil {
		return Trade{}, err
	}
	return closeWithClient(ctx, credentials, client, orderNo)
}

func closeWithClient(ctx context.Context, credentials Credentials, client *sdk.Client, orderNo string) (Trade, error) {
	if err := validateOrderNo(ctx, orderNo); err != nil {
		return Trade{}, err
	}
	response, err := client.TradeClose(ctx, gopay.BodyMap{"out_trade_no": orderNo})
	if err != nil {
		if ctx.Err() != nil {
			return Trade{}, ctx.Err()
		}
		return Trade{}, ErrCloseGateway
	}
	if response == nil || response.Response == nil || response.Response.OutTradeNo != orderNo || response.Response.TradeNo == "" {
		return Trade{}, ErrTradeInvalid
	}
	return Trade{OrderNo: response.Response.OutTradeNo, TradeNo: response.Response.TradeNo, AppID: credentials.AppID}, nil
}

// VerifyNotify parses and verifies one complete Alipay form notification.
// Every field must occur once; this prevents ambiguous values from being
// signed and then interpreted differently by downstream code.
func VerifyNotify(ctx context.Context, credentials Credentials, values url.Values) (Notify, error) {
	if err := contextError(ctx); err != nil {
		return Notify{}, err
	}
	if len(values) == 0 || len(values) > 64 || valuesByteSize(values) > maxNotifyBytes {
		return Notify{}, ErrNotifyInvalid
	}
	body := make(gopay.BodyMap, len(values))
	for name, entries := range values {
		if !validNotifyIdentifier(name, 128) || len(entries) != 1 || len(name)+len(entries[0]) > maxNotifyBytes {
			return Notify{}, ErrNotifyInvalid
		}
		body.Set(name, entries[0])
	}
	if body.GetString("sign_type") != sdk.RSA2 || body.GetString("sign") == "" {
		return Notify{}, ErrNotifySignature
	}
	valid, err := sdk.VerifySignWithCert([]byte(credentials.AlipayCertPEM), body)
	if err != nil || !valid {
		return Notify{}, ErrNotifySignature
	}
	if err := contextError(ctx); err != nil {
		return Notify{}, err
	}
	orderNo, tradeNo, appID, status, total := body.GetString("out_trade_no"), body.GetString("trade_no"), body.GetString("app_id"), body.GetString("trade_status"), body.GetString("total_amount")
	notifyID := body.GetString("notify_id")
	seller := body.GetString("seller_id")
	if !validNotifyIdentifier(orderNo, 32) || !validNotifyIdentifier(appID, 64) || !validNotifyIdentifier(seller, 64) || !validNotifyIdentifier(status, 64) ||
		(notifyID != "" && !validNotifyIdentifier(notifyID, 128)) || (tradeNo != "" && !validNotifyIdentifier(tradeNo, 64)) ||
		((status == "TRADE_SUCCESS" || status == "TRADE_FINISHED") && tradeNo == "") {
		return Notify{}, ErrNotifyInvalid
	}
	amount, err := money.ParseRMBCents(total)
	if err != nil || amount <= 0 {
		return Notify{}, ErrNotifyInvalid
	}
	if _, err := money.CentsToUnits(amount); err != nil {
		return Notify{}, ErrNotifyInvalid
	}
	var paidAt *time.Time
	if paid := body.GetString("gmt_payment"); paid != "" {
		paidAt, err = parseProviderTime(paid)
		if err != nil {
			return Notify{}, ErrNotifyInvalid
		}
	}
	// Keep signed identity mismatches intact for the callback evidence and order
	// checks. Signature verification is not an assertion of merchant ownership.
	return Notify{NotifyID: notifyID, OrderNo: orderNo, TradeNo: tradeNo, AppID: appID, SellerID: &seller, AmountCents: amount, Status: status, PaidAt: paidAt}, nil
}

func validNotifyIdentifier(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e || character == '&' || character == '=' {
			return false
		}
	}
	return true
}

func validateCheckoutInput(ctx context.Context, input CheckoutInput) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := validateOrderNo(ctx, input.OrderNo); err != nil {
		return err
	}
	if input.Subject == "" || len(input.Subject) > 256 || strings.TrimSpace(input.Subject) == "" || strings.ContainsAny(input.Subject, "\r\n") || input.AmountCents <= 0 {
		return ErrCheckoutInvalid
	}
	if _, err := parseReturnURL(input.ReturnURL); err != nil {
		return ErrCheckoutInvalid
	}
	if input.Method != payment.PayMethodWeb && input.Method != payment.PayMethodH5 {
		return ErrUnsupportedPayMethod
	}
	return nil
}

func validateOrderNo(ctx context.Context, orderNo string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if orderNo == "" || len(orderNo) > 128 || strings.TrimSpace(orderNo) != orderNo || strings.ContainsAny(orderNo, "\r\n&=") {
		return ErrTradeInvalid
	}
	return nil
}

func parseReturnURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, errors.New("return URL invalid")
	}
	return parsed, nil
}

func parseProviderTime(raw string) (*time.Time, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		var parsed time.Time
		if layout == time.RFC3339 || layout == time.RFC3339Nano {
			parsed, err = time.Parse(layout, raw)
		} else {
			parsed, err = time.ParseInLocation(layout, raw, location)
		}
		if err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("provider time invalid")
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("payment context is required")
	}
	return ctx.Err()
}

func valuesByteSize(values url.Values) int {
	total := 0
	for name, entries := range values {
		total += len(name)
		for _, value := range entries {
			total += len(value)
		}
	}
	return total
}
