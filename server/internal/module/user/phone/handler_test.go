package phone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type handlerServiceStub struct {
	actor     Actor
	sendInput SendCodeInput
	bindInput BindOrChangeInput
}

func (s *handlerServiceStub) SendCode(_ context.Context, actor Actor, input SendCodeInput) (SendCodeResult, error) {
	s.actor, s.sendInput = actor, input
	return SendCodeResult{ChallengeID: "challenge-1", ResendAfterSeconds: 60}, nil
}

func (s *handlerServiceStub) BindOrChange(_ context.Context, actor Actor, input BindOrChangeInput) (PhoneResult, error) {
	s.actor, s.bindInput = actor, input
	return PhoneResult{Phone: "+8615671628271"}, nil
}

func TestHandlerStrictlyBindsPhoneRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &handlerServiceStub{}
	handler := NewHandler(service, func(*gin.Context) (Actor, bool) {
		return Actor{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"}, true
	})
	router := gin.New()
	routes := router.Group("/api/admin/v1")
	routes.POST("/user/phone/send-code", handler.SendCode)
	routes.PUT("/user/phone", handler.BindOrChange)

	request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/user/phone/send-code", strings.NewReader(`{"target":"next","phone":"15671628271","challengeId":"challenge-1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actor.UserID != 7 || service.sendInput.Target != TargetNext || service.sendInput.Phone == nil {
		t.Fatalf("send response=%d actor=%+v input=%+v body=%s", response.Code, service.actor, service.sendInput, response.Body)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/admin/v1/user/phone", strings.NewReader(`{"nextPhone":"15671628271","nextChallengeId":"next","nextCode":"123456"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.bindInput.NextPhone != "15671628271" || service.bindInput.NextCode != "123456" {
		t.Fatalf("bind response=%d input=%+v body=%s", response.Code, service.bindInput, response.Body)
	}

	for _, body := range []string{
		`{"target":"current","phone":"15671628271"}`,
		`{"target":"next","phone":"15671628271","unknown":true}`,
		`{"target":"other","phone":"15671628271"}`,
	} {
		request = httptest.NewRequest(http.MethodPost, "/api/admin/v1/user/phone/send-code", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body)
		}
	}
}
