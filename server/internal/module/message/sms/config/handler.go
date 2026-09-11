package config

import (
	"context"
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type service interface {
	Load(context.Context) (Safe, error)
	Update(context.Context, Input) (Safe, error)
	Delete(context.Context) error
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

type updateRequest struct {
	SecretID   *string      `json:"secretId"`
	SecretKey  *string      `json:"secretKey"`
	SDKAppID   *string      `json:"smsSdkAppId"`
	SignName   *string      `json:"signName"`
	Region     *string      `json:"region"`
	Endpoint   *string      `json:"endpoint"`
	TTLMinutes *int         `json:"ttlMinutes"`
	IsEnabled  *yesno.Value `json:"isEnabled"`
}

func (r updateRequest) input() (Input, error) {
	if r.SecretID == nil || r.SecretKey == nil || r.SDKAppID == nil || r.SignName == nil ||
		r.Region == nil || r.Endpoint == nil || r.TTLMinutes == nil || r.IsEnabled == nil {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms config fields are required"))
	}
	return Input{
		SecretID:   *r.SecretID,
		SecretKey:  *r.SecretKey,
		SDKAppID:   *r.SDKAppID,
		SignName:   *r.SignName,
		Region:     *r.Region,
		Endpoint:   *r.Endpoint,
		TTLMinutes: *r.TTLMinutes,
		IsEnabled:  *r.IsEnabled,
	}, nil
}

func (h *Handler) Load(c *gin.Context) {
	safe, err := h.service.Load(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, safe)
}

func (h *Handler) Update(c *gin.Context) {
	var request updateRequest
	if err := validate.BindJSON(c, &request); err != nil {
		response.Fail(c, err)
		return
	}
	input, err := request.input()
	if err != nil {
		response.Fail(c, err)
		return
	}
	safe, err := h.service.Update(c.Request.Context(), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, safe)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := validate.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, struct{}{})
}
