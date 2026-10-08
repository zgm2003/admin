package mail

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestProviderFailureReturnsLocalizedMailErrorWithoutDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		code string
		zh   string
		en   string
	}{
		{"InternalError", "邮件发送失败，请稍后重试；如持续失败，请联系管理员", "Email could not be sent. Please try again later or contact an administrator if the problem persists"},
		{"timeout", "邮件发送超时，请稍后重试", "Email sending timed out. Please try again later"},
		{"FailedOperation.EmailAddrInBlacklist", "邮件服务商拒绝向该邮箱发送邮件，请检查邮箱地址或更换邮箱", "The email provider rejected this recipient. Please check the email address or use another one"},
		{"InvalidParameterValue.ReceiverEmailInvalid", "邮件服务商拒绝向该邮箱发送邮件，请检查邮箱地址或更换邮箱", "The email provider rejected this recipient. Please check the email address or use another one"},
		{"FailedOperation.ReceiverHasUnsubscribed", "邮件服务商拒绝向该邮箱发送邮件，请检查邮箱地址或更换邮箱", "The email provider rejected this recipient. Please check the email address or use another one"},
		{"FailedOperation.RejectedByRecipients", "邮件服务商拒绝向该邮箱发送邮件，请检查邮箱地址或更换邮箱", "The email provider rejected this recipient. Please check the email address or use another one"},
		{"FailedOperation.FrequencyLimit", "邮件服务商发送频率受限，请稍后重试", "The email provider is rate limiting delivery. Please try again later"},
		{"RequestLimitExceeded", "邮件服务商发送频率受限，请稍后重试", "The email provider is rate limiting delivery. Please try again later"},
		{"FailedOperation.InsufficientBalance", "邮件发送服务暂不可用，请联系管理员处理", "The email sending service is unavailable. Please contact an administrator"},
		{"FailedOperation.InsufficientQuota", "邮件发送服务暂不可用，请联系管理员处理", "The email sending service is unavailable. Please contact an administrator"},
		{"FailedOperation.InvalidTemplateID", "邮件发送服务暂不可用，请联系管理员处理", "The email sending service is unavailable. Please contact an administrator"},
		{"FailedOperation.NotAuthenticatedSender", "邮件发送服务暂不可用，请联系管理员处理", "The email sending service is unavailable. Please contact an administrator"},
		{"AuthFailure.SignatureFailure", "邮件发送失败，请稍后重试；如持续失败，请联系管理员", "Email could not be sent. Please try again later or contact an administrator if the problem persists"},
		{"Future.UnknownError", "邮件发送失败，请稍后重试；如持续失败，请联系管理员", "Email could not be sent. Please try again later or contact an administrator if the problem persists"},
		{"", "邮件发送失败，请稍后重试；如持续失败，请联系管理员", "Email could not be sent. Please try again later or contact an administrator if the problem persists"},
	} {
		t.Run(test.code, func(t *testing.T) {
			cause := NewProviderError(test.code, "private-provider-diagnostic SECRET_KEY verification-code")
			err := providerFailure(fmt.Errorf("send verification mail: %w", cause))
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.HTTPStatus != 503 || appErr.Code != 18001 {
				t.Fatalf("error = %+v, want mail sending error 503/18001", appErr)
			}
			if !errors.Is(err, cause) {
				t.Fatal("provider diagnostic was lost from the internal error chain")
			}
			for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
				want := test.zh
				if locale == i18n.EnUS {
					want = test.en
				}
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				request := httptest.NewRequest(http.MethodPost, "/send-code", nil)
				ctx.Request = request.WithContext(i18n.WithLocale(request.Context(), locale))
				response.Fail(ctx, err)
				body := recorder.Body.String()
				if recorder.Code != 503 || !strings.Contains(body, `"code":18001`) || !strings.Contains(body, want) || !strings.Contains(body, `"data":null`) {
					t.Fatalf("locale=%s status=%d body=%s", locale, recorder.Code, body)
				}
				if strings.Contains(body, "private-provider") || strings.Contains(body, "SECRET_KEY") || strings.Contains(body, "verification-code") || (test.code != "" && strings.Contains(body, test.code)) {
					t.Fatalf("response leaked provider diagnostics: %s", body)
				}
			}
		})
	}
}
