package recipientrule

import (
	"fmt"
	"net/http"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ImportTemplate(ctx *gin.Context) {
	value, err := h.service.ImportTemplate(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) PreviewXlsx(ctx *gin.Context) {
	content, err := bindXlsxImportInput(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.PreviewXlsx(ctx.Request.Context(), content)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) ImportXlsx(ctx *gin.Context) {
	content, err := bindXlsxImportInput(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.ImportXlsx(ctx.Request.Context(), content)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) ExportXlsx(ctx *gin.Context) {
	value, err := h.service.ExportXlsx(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func bindXlsxImportInput(ctx *gin.Context) (XlsxImportInput, error) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 4*((XlsxMaxBytes+2)/3)+2048)
	var in XlsxImportInput
	if err := validate.BindJSON(ctx, &in); err != nil {
		return XlsxImportInput{}, err
	}
	if _, err := decodeXlsxImportInput(in); err != nil {
		return XlsxImportInput{}, apperror.InvalidRequest(err)
	}
	return in, nil
}

func (h *Handler) List(ctx *gin.Context) {
	values, err := h.service.List(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, values)
}

func (h *Handler) Create(ctx *gin.Context) {
	input, err := bindInput(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	id, err := h.service.Create(ctx.Request.Context(), input)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) Update(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	input, err := bindInput(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	if err = h.service.Update(ctx.Request.Context(), id, input); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}

func (h *Handler) SetStatus(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	var request struct {
		IsEnabled *yesno.Value `json:"isEnabled"`
	}
	if err = validate.BindJSON(ctx, &request); err != nil {
		response.Fail(ctx, err)
		return
	}
	if request.IsEnabled == nil {
		response.Fail(ctx, apperror.InvalidRequest(fmt.Errorf("isEnabled is required")))
		return
	}
	if err = h.service.SetStatus(ctx.Request.Context(), id, *request.IsEnabled); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{"id": id, "isEnabled": *request.IsEnabled})
}

func (h *Handler) Delete(ctx *gin.Context) {
	id, err := validate.ParsePositiveInt64(ctx.Param("id"), "id")
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	if err = h.service.Delete(ctx.Request.Context(), id); err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, map[string]any{})
}

func bindInput(ctx *gin.Context) (Input, error) {
	var request struct {
		Scope     *Scope       `json:"scope" binding:"required"`
		Pattern   string       `json:"pattern"`
		Action    *Action      `json:"action" binding:"required"`
		Name      string       `json:"name"`
		Remark    string       `json:"remark"`
		IsEnabled *yesno.Value `json:"isEnabled" binding:"required"`
	}
	if err := validate.BindJSON(ctx, &request); err != nil {
		return Input{}, err
	}
	if !request.Scope.IsValid() || !request.Action.IsValid() || !yesno.IsValid(*request.IsEnabled) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("recipient rule enum is invalid"))
	}
	return Input{Scope: *request.Scope, Pattern: request.Pattern, Action: *request.Action, Name: request.Name, Remark: request.Remark, IsEnabled: *request.IsEnabled}, nil
}
