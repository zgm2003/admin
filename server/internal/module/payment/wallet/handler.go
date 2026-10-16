package wallet

import (
	"context"
	"net/http"
	"net/url"

	"admin/server/internal/authcontext"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"github.com/gin-gonic/gin"
)

type handlerService interface {
	Get(context.Context, int64) (Value, error)
	AdminGet(context.Context, int64) (AdminValue, error)
	AdminList(context.Context, int, int) (pagination.Result[AdminValue], error)
}

type Handler struct{ service handlerService }

func NewHandler(service handlerService) *Handler { return &Handler{service: service} }

func (h *Handler) Current(c *gin.Context) {
	identity, ok := authcontext.Get(c)
	if !ok || identity.UserID < 1 || identity.PlatformID < 1 {
		response.Fail(c, apperror.Unauthorized(nil))
		return
	}
	if c.Request.URL.RawQuery != "" {
		response.Fail(c, apperror.InvalidRequest(nil))
		return
	}
	value, err := h.service.Get(c.Request.Context(), identity.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, value.PersonalValue)
}

func (h *Handler) AdminList(c *gin.Context) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	query, err := parseWalletAdminQuery(values)
	if err != nil {
		response.Fail(c, err)
		return
	}
	value, err := h.service.AdminList(c.Request.Context(), query.Page, query.PageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, value)
}

func (h *Handler) AdminDetail(c *gin.Context) {
	userID, err := validate.ParsePositiveInt64(c.Param("userId"), "userId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	value, err := h.service.AdminGet(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, value)
}

type walletAdminQuery struct{ Page, PageSize int }

func parseWalletAdminQuery(values url.Values) (walletAdminQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize") || len(entries) != 1 {
			return walletAdminQuery{}, apperror.InvalidRequest(nil)
		}
	}
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return walletAdminQuery{}, err
	}
	if (request.Page - 1) > int(^uint(0)>>1)/request.PageSize {
		return walletAdminQuery{}, apperror.InvalidRequest(nil)
	}
	return walletAdminQuery{Page: request.Page, PageSize: request.PageSize}, nil
}
