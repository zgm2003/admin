package recipientrule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode/utf8"

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

func (h *Handler) PreviewCSV(ctx *gin.Context) {
	content, err := bindCSVContent(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.PreviewCSV(ctx.Request.Context(), content)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) ImportCSV(ctx *gin.Context) {
	content, err := bindCSVContent(ctx)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	value, err := h.service.ImportCSV(ctx.Request.Context(), content)
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func (h *Handler) ExportCSV(ctx *gin.Context) {
	value, err := h.service.ExportCSV(ctx.Request.Context())
	if err != nil {
		response.Fail(ctx, err)
		return
	}
	response.OK(ctx, http.StatusOK, value)
}

func bindCSVContent(ctx *gin.Context) (string, error) {
	// JSON escaping can expand a CSV byte up to six bytes.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 6*CSVMaxBytes+1024)
	payload, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return "", apperror.InvalidRequest(err)
	}
	// encoding/json otherwise silently substitutes U+FFFD for invalid bytes.
	if !utf8.Valid(payload) {
		return "", apperror.InvalidRequest(fmt.Errorf("CSV JSON must be valid UTF-8"))
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(payload))
	var input struct {
		Content *string `json:"content" binding:"required"`
	}
	if err := validate.BindJSON(ctx, &input); err != nil {
		return "", err
	}
	var raw struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return "", apperror.InvalidRequest(err)
	}
	if !validCSVJSONUnicode(raw.Content) {
		return "", apperror.InvalidRequest(fmt.Errorf("CSV JSON has an unpaired Unicode surrogate"))
	}
	return *input.Content, nil
}

// The JSON binder already checked syntax and field shape. Inspect only escaped
// Unicode in the original content string, before JSON replacement can hide it.
func validCSVJSONUnicode(raw []byte) bool {
	for index := 1; index < len(raw)-1; index++ {
		if raw[index] != '\\' {
			continue
		}
		index++
		if raw[index] != 'u' {
			continue
		}
		value, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
		if err != nil {
			return false
		}
		index += 4
		if value >= 0xDC00 && value <= 0xDFFF {
			return false
		}
		if value < 0xD800 || value > 0xDBFF {
			continue
		}
		if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
		if err != nil || low < 0xDC00 || low > 0xDFFF {
			return false
		}
		index += 6
	}
	return true
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
	var input Input
	if err := validate.BindJSON(ctx, &input); err != nil {
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
	var input Input
	if err = validate.BindJSON(ctx, &input); err != nil {
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
