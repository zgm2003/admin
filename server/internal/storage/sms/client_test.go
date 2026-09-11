package sms

import (
	"context"
	"errors"
	"strings"
	"testing"

	messagesms "admin/server/internal/module/message/sms"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	sdk "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const testPhone = "+8615671628271"

type fakeSendAPI struct {
	response *sdk.SendSmsResponse
	err      error
	request  *sdk.SendSmsRequest
}

func (f *fakeSendAPI) SendSmsWithContext(_ context.Context, request *sdk.SendSmsRequest) (*sdk.SendSmsResponse, error) {
	f.request = request
	return f.response, f.err
}

func testClient(api sendAPI) *Client {
	return &Client{
		newSendAPI: func(*common.Credential, string, string) (sendAPI, error) { return api, nil },
	}
}

func testInput() messagesms.SendInput {
	return messagesms.SendInput{
		Region:         "ap-guangzhou",
		Endpoint:       "sms.tencentcloudapi.com",
		SecretID:       "secret-id",
		SecretKey:      "secret-key",
		SDKAppID:       "1400006666",
		SignName:       "sign",
		TemplateID:     "123456",
		ToPhone:        testPhone,
		TemplateParams: []string{"123456", "5"},
	}
}

func okResponse() *sdk.SendSmsResponse {
	return &sdk.SendSmsResponse{Response: &sdk.SendSmsResponseParams{
		RequestId: common.StringPtr("request-1"),
		SendStatusSet: []*sdk.SendStatus{{
			Code:        common.StringPtr("Ok"),
			PhoneNumber: common.StringPtr(testPhone),
			SerialNo:    common.StringPtr("serial-1"),
			Fee:         common.Uint64Ptr(1),
		}},
	}}
}

func providerFailure(err error) *messagesms.ProviderError {
	var failure *messagesms.ProviderError
	if errors.As(err, &failure) {
		return failure
	}
	return nil
}

func TestSendBuildsSingleRecipientRequestAndMapsResult(t *testing.T) {
	api := &fakeSendAPI{response: okResponse()}
	result, err := testClient(api).Send(context.Background(), testInput())
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.RequestID != "request-1" || result.SerialNo != "serial-1" || result.Fee != 1 {
		t.Fatalf("result = %+v", result)
	}
	if api.request == nil {
		t.Fatal("provider request was not sent")
	}
	if len(api.request.PhoneNumberSet) != 1 || value(api.request.PhoneNumberSet[0]) != testPhone {
		t.Fatalf("phone set = %v", api.request.PhoneNumberSet)
	}
	if value(api.request.SmsSdkAppId) != "1400006666" || value(api.request.SignName) != "sign" || value(api.request.TemplateId) != "123456" {
		t.Fatalf("request = %+v", api.request)
	}
	if len(api.request.TemplateParamSet) != 2 || value(api.request.TemplateParamSet[0]) != "123456" || value(api.request.TemplateParamSet[1]) != "5" {
		t.Fatalf("template params = %v", api.request.TemplateParamSet)
	}
}

func TestSendRejectsUnsafeProviderOutcomes(t *testing.T) {
	for _, test := range []struct {
		name     string
		response *sdk.SendSmsResponse
		wantCode string
	}{
		{name: "nil response", response: nil, wantCode: "empty_response"},
		{name: "nil response body", response: &sdk.SendSmsResponse{}, wantCode: "empty_response"},
		{name: "empty status set", response: &sdk.SendSmsResponse{Response: &sdk.SendSmsResponseParams{RequestId: common.StringPtr("request-1")}}, wantCode: "invalid_status_set"},
		{
			name: "multiple statuses",
			response: &sdk.SendSmsResponse{Response: &sdk.SendSmsResponseParams{
				RequestId: common.StringPtr("request-1"),
				SendStatusSet: []*sdk.SendStatus{
					{Code: common.StringPtr("Ok"), PhoneNumber: common.StringPtr(testPhone), SerialNo: common.StringPtr("serial-1")},
					{Code: common.StringPtr("Ok"), PhoneNumber: common.StringPtr(testPhone), SerialNo: common.StringPtr("serial-2")},
				},
			}},
			wantCode: "invalid_status_set",
		},
		{name: "phone mismatch", response: responseFor(func(status *sdk.SendStatus) {
			status.PhoneNumber = common.StringPtr("+8613800000000")
		}), wantCode: "phone_mismatch"},
		{name: "missing status code", response: responseFor(func(status *sdk.SendStatus) {
			status.Code = nil
		}), wantCode: "sms_missing_status_code"},
		{name: "non Ok status", response: responseFor(func(status *sdk.SendStatus) {
			status.Code = common.StringPtr("Failed")
			status.Message = common.StringPtr("template not approved")
		}), wantCode: "sms_failed"},
		{
			name: "missing request id",
			response: &sdk.SendSmsResponse{Response: &sdk.SendSmsResponseParams{
				SendStatusSet: []*sdk.SendStatus{{
					Code:        common.StringPtr("Ok"),
					PhoneNumber: common.StringPtr(testPhone),
					SerialNo:    common.StringPtr("serial-1"),
				}},
			}},
			wantCode: "missing_request_id",
		},
		{name: "missing serial number", response: responseFor(func(status *sdk.SendStatus) {
			status.SerialNo = nil
		}), wantCode: "missing_serial_no"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := testClient(&fakeSendAPI{response: test.response}).Send(context.Background(), testInput())
			failure := providerFailure(err)
			if failure == nil {
				t.Fatalf("Send() accepted %s", test.name)
			}
			if failure.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", failure.Code, test.wantCode)
			}
			if strings.Contains(failure.Summary, "secret-key") {
				t.Fatalf("error summary leaked a credential: %q", failure.Summary)
			}
		})
	}
}

func TestSendMapsProviderFailuresWithoutLeakingSecrets(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := testClient(&fakeSendAPI{err: ctx.Err()}).Send(ctx, testInput())
		failure := providerFailure(err)
		if failure == nil || failure.Code != "timeout" {
			t.Fatalf("canceled send error = %v", err)
		}
	})

	t.Run("sdk error", func(t *testing.T) {
		sdkError := &tcerrors.TencentCloudSDKError{
			Code:      "AuthFailure.SecretIdNotFound",
			Message:   "AKID1234567890 is invalid for +8615671628271",
			RequestId: "request-9",
		}
		_, err := testClient(&fakeSendAPI{err: sdkError}).Send(context.Background(), testInput())
		failure := providerFailure(err)
		if failure == nil || failure.Code != "sms_authfailure.secretidnotfound" {
			t.Fatalf("sdk send error = %v", err)
		}
		if strings.Contains(failure.Summary, "AKID1234567890") || strings.Contains(failure.Summary, "15671628271") {
			t.Fatalf("error summary leaked provider identifiers: %q", failure.Summary)
		}
	})
}

func TestSendRejectsIncompleteConfiguration(t *testing.T) {
	for _, test := range []struct {
		name     string
		mutate   func(*messagesms.SendInput)
		wantCode string
	}{
		{name: "missing region", mutate: func(input *messagesms.SendInput) { input.Region = "" }, wantCode: "invalid_configuration"},
		{name: "missing secret id", mutate: func(input *messagesms.SendInput) { input.SecretID = "" }, wantCode: "invalid_configuration"},
		{name: "missing secret key", mutate: func(input *messagesms.SendInput) { input.SecretKey = "" }, wantCode: "invalid_configuration"},
		{name: "missing sdk app id", mutate: func(input *messagesms.SendInput) { input.SDKAppID = "" }, wantCode: "invalid_configuration"},
		{name: "missing sign name", mutate: func(input *messagesms.SendInput) { input.SignName = "" }, wantCode: "invalid_configuration"},
		{name: "missing phone", mutate: func(input *messagesms.SendInput) { input.ToPhone = "" }, wantCode: "invalid_configuration"},
		{name: "missing template id", mutate: func(input *messagesms.SendInput) { input.TemplateID = "" }, wantCode: "invalid_configuration"},
		{name: "missing template parameters", mutate: func(input *messagesms.SendInput) { input.TemplateParams = nil }, wantCode: "invalid_configuration"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := testInput()
			test.mutate(&input)
			api := &fakeSendAPI{response: okResponse()}
			_, err := testClient(api).Send(context.Background(), input)
			failure := providerFailure(err)
			if failure == nil || failure.Code != test.wantCode {
				t.Fatalf("error = %v, want code %q", err, test.wantCode)
			}
			if api.request != nil {
				t.Fatal("incomplete configuration reached the provider")
			}
		})
	}
}

func responseFor(mutate func(*sdk.SendStatus)) *sdk.SendSmsResponse {
	response := okResponse()
	mutate(response.Response.SendStatusSet[0])
	return response
}
