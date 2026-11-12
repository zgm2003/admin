package run

import (
	"net/http"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"

	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Send(c *gin.Context) {
	identity := mustIdentity(c)
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request sendMessageRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.Send(c.Request.Context(), identity, id, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusAccepted, result)
}

func (h *Handler) Cancel(c *gin.Context) {
	identity := mustIdentity(c)
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err = validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err = h.service.Cancel(c.Request.Context(), identity, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) RunsOwn(c *gin.Context) { h.runs(c, false) }
func (h *Handler) RunsAll(c *gin.Context) { h.runs(c, true) }

func (h *Handler) runs(c *gin.Context, viewAll bool) {
	identity := mustIdentity(c)
	query, err := parseRunQuery(c.Request.URL.Query(), viewAll)
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.Runs(c.Request.Context(), identity, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) RunDetailOwn(c *gin.Context) { h.runDetail(c, false) }
func (h *Handler) RunDetailAll(c *gin.Context) { h.runDetail(c, true) }

func (h *Handler) runDetail(c *gin.Context, viewAll bool) {
	identity := mustIdentity(c)
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.RunDetail(c.Request.Context(), identity, id, viewAll)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func mustIdentity(c *gin.Context) auth.Identity {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		panic("run handler requires authenticated identity")
	}
	return identity
}
