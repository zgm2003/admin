package chat

import (
	"errors"
	"net/http"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"

	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Agents(c *gin.Context) {
	if _, ok := auth.IdentityFromContext(c); !ok {
		response.Fail(c, apperror.Unauthorized(errors.New("authentication identity is missing")))
		return
	}
	items, err := h.service.Agents(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct {
		List []AgentValue `json:"list"`
	}{List: items})
}

func (h *Handler) Conversations(c *gin.Context) {
	identity := mustIdentity(c)
	query, err := parseConversationQuery(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.Conversations(c.Request.Context(), identity, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	identity := mustIdentity(c)
	var request createConversationRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	id, err := h.service.CreateConversation(c.Request.Context(), identity, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, struct {
		ID int64 `json:"id"`
	}{ID: id})
}

func (h *Handler) RenameConversation(c *gin.Context) {
	identity := mustIdentity(c)
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var request renameConversationRequest
	if err = validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	if err = h.service.RenameConversation(c.Request.Context(), identity, id, input); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) DeleteConversation(c *gin.Context) {
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
	if err = h.service.DeleteConversation(c.Request.Context(), identity, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}

func (h *Handler) Messages(c *gin.Context) {
	identity := mustIdentity(c)
	id, err := validate.ParsePositiveInt64(c.Param("id"), "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	before, limit, err := parseMessageLimit(c.Request.URL.Query())
	if err != nil {
		response.Fail(c, invalid(err))
		return
	}
	result, err := h.service.Messages(c.Request.Context(), identity, id, before, limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func (h *Handler) Balance(c *gin.Context) {
	identity := mustIdentity(c)
	result, err := h.service.Balance(c.Request.Context(), identity.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}

func mustIdentity(c *gin.Context) auth.Identity {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		panic("chat handler requires authenticated identity")
	}
	return identity
}
