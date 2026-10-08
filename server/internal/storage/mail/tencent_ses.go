package mail

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	messagemail "admin/server/internal/module/message/mail"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ses "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ses/v20201002"
)

type Client struct{ httpClient *http.Client }

// The SDK flattens network errors into TencentCloudSDKError and drops Unwrap.
// Capture timeout semantics at the transport boundary, independently per Send.
type sendTransport struct {
	base     http.RoundTripper
	timedOut atomic.Bool
}

func (t *sendTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	var timeout net.Error
	t.timedOut.Store(errors.As(err, &timeout) && timeout.Timeout())
	return response, err
}

func NewTencentSESClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{httpClient: httpClient}
}

func (c *Client) Send(ctx context.Context, input messagemail.SendInput) (messagemail.ProviderSendResult, error) {
	if input.TemplateID <= 0 {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("invalid_template", "template id is invalid")
	}
	data, err := json.Marshal(input.TemplateData)
	if err != nil {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("invalid_template_data", err.Error())
	}
	cred := common.NewCredential(input.SecretID, input.SecretKey)
	cp := profile.NewClientProfile()
	cp.HttpProfile.ReqTimeout = 8
	if input.Endpoint != "" {
		cp.HttpProfile.Endpoint = input.Endpoint
	}
	client, err := ses.NewClient(cred, input.Region, cp)
	if err != nil {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("client_init", err.Error())
	}
	transport := &sendTransport{base: http.DefaultTransport}
	if c.httpClient != nil && c.httpClient.Transport != nil {
		transport.base = c.httpClient.Transport
	}
	client.WithHttpTransport(transport)
	req := ses.NewSendEmailRequest()
	req.Template = &ses.Template{TemplateID: common.Uint64Ptr(uint64(input.TemplateID)), TemplateData: common.StringPtr(string(data))}
	req.Destination = []*string{common.StringPtr(input.ToEmail)}
	fromAddress := input.FromEmail
	if input.FromName != "" {
		fromAddress = input.FromName + " <" + input.FromEmail + ">"
	}
	req.FromEmailAddress = common.StringPtr(fromAddress)
	req.Subject = common.StringPtr(input.Subject)
	if input.ReplyTo != "" {
		req.ReplyToAddresses = common.StringPtr(input.ReplyTo)
	}
	result, err := client.SendEmailWithContext(ctx, req)
	if err != nil {
		if ctx.Err() != nil || transport.timedOut.Load() {
			return messagemail.ProviderSendResult{}, messagemail.NewProviderError("timeout", "email provider request timed out")
		}
		code := "ses_error"
		var sdkError *tcerrors.TencentCloudSDKError
		if errors.As(err, &sdkError) && strings.TrimSpace(sdkError.Code) != "" {
			code = sdkError.Code
		}
		// Keep the upstream code and request ID for administrator diagnostics,
		// but never persist credentials or verification codes from error text.
		summary := err.Error()
		for _, secret := range []string{input.SecretID, input.SecretKey, input.TemplateData["code"]} {
			if secret != "" {
				summary = strings.ReplaceAll(summary, secret, "[REDACTED]")
			}
		}
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError(code, summary)
	}
	if result == nil || result.Response == nil {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("empty_response", "email provider returned an empty response")
	}
	if strings.TrimSpace(value(result.Response.RequestId)) == "" || strings.TrimSpace(value(result.Response.MessageId)) == "" {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("invalid_response", "email provider returned an incomplete delivery receipt")
	}
	return messagemail.ProviderSendResult{RequestID: value(result.Response.RequestId), MessageID: value(result.Response.MessageId)}, nil
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
