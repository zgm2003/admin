package profile

import (
	"admin/server/internal/module/auth/login"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type avatarPermissionStub struct {
	allowed bool
	err     error
}

func (s avatarPermissionStub) Allowed(context.Context, auth.Identity, string) (bool, error) {
	return s.allowed, s.err
}

type avatarCopyStub struct{ calls int }

func (s *avatarCopyStub) PrepareAvatar(context.Context, auth.Identity, int64, string) (storagefile.AvatarCopy, error) {
	s.calls++
	return storagefile.AvatarCopy{}, errors.New("unexpected copy")
}
func TestAvatarCopyRequiresFilePermissionBeforeStorageIO(t *testing.T) {
	service := NewService(&Repository{})
	copier := &avatarCopyStub{}
	service.SetAvatarCopier(copier)
	service.SetAvatarPermissions(avatarPermissionStub{})
	id := int64(12)
	_, err := service.Update(context.Background(), 7, 7, Input{Username: "alice", AvatarFileID: &id, AvatarRequestKey: "d750d24b-23f8-4c10-a240-cd9865706268", PlatformID: 1, Identity: auth.Identity{UserID: 7, PlatformID: 1}})
	var app *apperror.Error
	if !errors.As(err, &app) || app.Code != apperror.CodeForbidden || copier.calls != 0 {
		t.Fatalf("err=%v copy calls=%d", err, copier.calls)
	}
}

func TestAvatarSelectionHandlerUsesAuthenticatedPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &profileServiceStub{updated: Value{UserID: 7}}
	router := gin.New()
	registerTestRoutes(router, stub, &passwordServiceStub{}, true)
	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/user/profile", strings.NewReader(`{"username":"alice","avatarFileId":12,"avatarRequestKey":"d750d24b-23f8-4c10-a240-cd9865706268"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || stub.input.PlatformID != 1 || stub.input.AvatarFileID == nil || *stub.input.AvatarFileID != 12 {
		t.Fatalf("status=%d input=%+v body=%s", recorder.Code, stub.input, recorder.Body)
	}
}

func TestAvatarSelectionRequiresPositiveFileAndUUIDRequest(t *testing.T) {
	id := int64(12)
	input, err := (updateRequest{Username: "alice", AvatarFileID: &id, AvatarRequestKey: "d750d24b-23f8-4c10-a240-cd9865706268"}).input()
	if err != nil || input.AvatarFileID == nil || *input.AvatarFileID != id {
		t.Fatalf("selection input=%+v err=%v", input, err)
	}
	for _, request := range []updateRequest{
		{Username: "alice", AvatarFileID: &id},
		{Username: "alice", AvatarRequestKey: "d750d24b-23f8-4c10-a240-cd9865706268"},
		{Username: "alice", AvatarFileID: &id, AvatarRequestKey: "invalid"},
		{Username: "alice", AvatarFileID: &id, AvatarRequestKey: "d750d24b-23f8-4c10-a240-cd9865706268", Avatar: "avatar/old.png"},
	} {
		if _, err := request.input(); err == nil {
			t.Fatalf("invalid selection accepted: %+v", request)
		}
	}
}
