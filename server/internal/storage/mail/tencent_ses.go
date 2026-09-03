package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	messagemail "admin/server/internal/module/message/mail"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ses "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ses/v20201002"
)

type Client struct{ httpClient *http.Client }

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
	if c.httpClient != nil && c.httpClient.Transport != nil {
		client.WithHttpTransport(c.httpClient.Transport)
	}
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
		if ctx.Err() != nil {
			return messagemail.ProviderSendResult{}, messagemail.NewProviderError("timeout", "email provider request timed out")
		}
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("ses_error", fmt.Sprintf("%v", err))
	}
	if result == nil || result.Response == nil {
		return messagemail.ProviderSendResult{}, messagemail.NewProviderError("empty_response", "email provider returned an empty response")
	}
	return messagemail.ProviderSendResult{RequestID: value(result.Response.RequestId), MessageID: value(result.Response.MessageId)}, nil
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
