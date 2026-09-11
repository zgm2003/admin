package sms

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	messagesms "admin/server/internal/module/message/sms"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sdk "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const okStatus = "Ok"

// sendAPI is the narrow SDK seam: it exposes exactly the one call SMS needs and
// lets tests replace the Tencent Cloud client with a fake.
type sendAPI interface {
	SendSmsWithContext(ctx context.Context, request *sdk.SendSmsRequest) (*sdk.SendSmsResponse, error)
}

type Client struct {
	httpClient *http.Client
	newSendAPI func(credential *common.Credential, region, endpoint string) (sendAPI, error)
}

func NewTencentSMSClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{httpClient: httpClient, newSendAPI: newSDKSendAPI}
}

func newSDKSendAPI(credential *common.Credential, region, endpoint string) (sendAPI, error) {
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.ReqTimeout = 8
	if endpoint != "" {
		clientProfile.HttpProfile.Endpoint = endpoint
	}
	return sdk.NewClient(credential, region, clientProfile)
}

// Send only accepts a fully configured single-recipient request; it never
// guesses a missing endpoint, credential or template.
func (c *Client) Send(ctx context.Context, input messagesms.SendInput) (messagesms.ProviderResult, error) {
	if strings.TrimSpace(input.Region) == "" ||
		strings.TrimSpace(input.SecretID) == "" ||
		strings.TrimSpace(input.SecretKey) == "" ||
		strings.TrimSpace(input.SDKAppID) == "" ||
		strings.TrimSpace(input.SignName) == "" ||
		strings.TrimSpace(input.TemplateID) == "" ||
		strings.TrimSpace(input.ToPhone) == "" ||
		len(input.TemplateParams) == 0 {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("invalid_configuration", "sms provider configuration is incomplete")
	}

	api, err := c.newSendAPI(common.NewCredential(input.SecretID, input.SecretKey), input.Region, input.Endpoint)
	if err != nil {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("client_init", "sms provider client could not be created")
	}

	request := sdk.NewSendSmsRequest()
	request.PhoneNumberSet = []*string{common.StringPtr(input.ToPhone)}
	request.SmsSdkAppId = common.StringPtr(input.SDKAppID)
	request.SignName = common.StringPtr(input.SignName)
	request.TemplateId = common.StringPtr(input.TemplateID)
	params := make([]*string, 0, len(input.TemplateParams))
	for _, parameter := range input.TemplateParams {
		params = append(params, common.StringPtr(parameter))
	}
	request.TemplateParamSet = params

	response, err := api.SendSmsWithContext(ctx, request)
	if err != nil {
		if ctx.Err() != nil {
			return messagesms.ProviderResult{}, messagesms.NewProviderError("timeout", "sms provider request timed out")
		}
		return messagesms.ProviderResult{}, sdkFailure(err)
	}
	return mapSendResult(input.ToPhone, response)
}

// mapSendResult trusts only a single status entry that matches the request.
func mapSendResult(toPhone string, response *sdk.SendSmsResponse) (messagesms.ProviderResult, error) {
	if response == nil || response.Response == nil {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("empty_response", "sms provider returned an empty response")
	}
	statuses := response.Response.SendStatusSet
	if len(statuses) != 1 || statuses[0] == nil {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("invalid_status_set", "sms provider must return exactly one status entry")
	}
	status := statuses[0]
	if value(status.PhoneNumber) != toPhone {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("phone_mismatch", "sms provider status does not match the requested number")
	}
	if code := value(status.Code); code != okStatus {
		if code == "" {
			code = "missing_status_code"
		}
		return messagesms.ProviderResult{}, messagesms.NewProviderError("sms_"+strings.ToLower(code), maskProviderText(value(status.Message)))
	}
	requestID := value(response.Response.RequestId)
	if requestID == "" {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("missing_request_id", "sms provider did not return a request id")
	}
	serialNo := value(status.SerialNo)
	if serialNo == "" {
		return messagesms.ProviderResult{}, messagesms.NewProviderError("missing_serial_no", "sms provider did not return a serial number")
	}
	var fee uint64
	if status.Fee != nil {
		fee = *status.Fee
	}
	return messagesms.ProviderResult{RequestID: requestID, SerialNo: serialNo, Fee: fee}, nil
}

func sdkFailure(err error) *messagesms.ProviderError {
	var sdkError *tcerrors.TencentCloudSDKError
	if errors.As(err, &sdkError) {
		code := strings.ToLower(strings.TrimSpace(sdkError.Code))
		if code == "" {
			code = "request_failed"
		}
		return messagesms.NewProviderError("sms_"+code, maskProviderText(sdkError.Message))
	}
	return messagesms.NewProviderError("sms_request_failed", maskProviderText(err.Error()))
}

var (
	accountIDPattern = regexp.MustCompile(`AKID[A-Za-z0-9]{6,}`)
	digitRunPattern  = regexp.MustCompile(`[0-9]{6,}`)
)

// maskProviderText removes account identifiers and number runs (phone numbers,
// serial numbers) from provider diagnostics before they reach logs or audit.
func maskProviderText(message string) string {
	return digitRunPattern.ReplaceAllString(accountIDPattern.ReplaceAllString(message, "AKID***"), "***")
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
