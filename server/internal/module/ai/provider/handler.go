package provider

import (
	"admin/server/internal/authcontext"
	price "admin/server/internal/module/ai/provider/model"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/validate"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"strconv"
	"time"
)

type Handler struct {
	service   *Service
	models    *price.ModelService
	catalog   *CatalogService
	priceSync *PriceSyncService
	prices    *price.Service
}

func NewHandler(s *Service, m *price.ModelService, c *CatalogService, sync *PriceSyncService, p *price.Service) *Handler {
	return &Handler{s, m, c, sync, p}
}
func actor(c *gin.Context) int64 {
	identity, ok := authcontext.Get(c)
	if !ok {
		return 0
	}
	return identity.UserID
}
func idParam(c *gin.Context, name string) (int64, error) {
	return validate.ParsePositiveInt64(c.Param(name), name)
}
func fail(c *gin.Context, err error) {
	var app *apperror.Error
	if errors.As(err, &app) {
		response.Fail(c, err)
		return
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, price.ErrModelNotFound) || errors.Is(err, price.ErrPriceNotFound) {
		response.Fail(c, apperror.NotFound(err))
		return
	}
	if errors.Is(err, ErrInvalidConfiguration) {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	var pg *pgconn.PgError
	if errors.Is(err, ErrReferenced) || errors.Is(err, ErrCatalogChanged) || errors.Is(err, price.ErrSourceVersionChanged) || errors.Is(err, price.ErrPublishedImmutable) || errors.Is(err, price.ErrPriceDigestMismatch) || errors.As(err, &pg) && pg.Code == "23505" {
		response.Fail(c, apperror.Conflict(i18n.KeyConflict, nil, err))
		return
	}
	if errors.Is(err, price.ErrIncompleteBillingPrice) {
		response.Fail(c, apperror.InvalidRequest(err))
		return
	}
	response.Fail(c, apperror.DependencyUnavailable(err))
}
func pagination(c *gin.Context) (int, int, error) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		return 0, 0, apperror.InvalidRequest(err)
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		return 0, 0, apperror.InvalidRequest(errors.New("pagination invalid"))
	}
	return page, size, nil
}
func (h *Handler) List(c *gin.Context) {
	p, s, err := pagination(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := h.service.List(c.Request.Context(), p, s)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": p, "pageSize": s})
}
func (h *Handler) Detail(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	result, err := h.service.Configuration(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, result)
}
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.service.Create(c.Request.Context(), req, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 201, out)
}
func (h *Handler) Update(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	var req UpdateRequest
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.service.Update(c.Request.Context(), id, req, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, out)
}
func (h *Handler) Status(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if err := h.service.SetStatus(c.Request.Context(), id, req.Status); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, actor(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) Candidates(c *gin.Context) {
	if c.Param("id") != "" {
		id, err := idParam(c, "id")
		if err != nil {
			fail(c, err)
			return
		}
		if _, err := h.service.Detail(c.Request.Context(), id); err != nil {
			fail(c, err)
			return
		}
	}
	p, s, err := pagination(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := h.service.Candidates(c.Request.Context(), c.Query("keyword"), p, s)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": p, "pageSize": s})
}
func (h *Handler) Test(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		fail(c, err)
		return
	}
	if h.catalog == nil {
		fail(c, errors.New("AI catalogue source unavailable"))
		return
	}
	out, err := h.catalog.Preview(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"connected": true, "modelCount": len(out.Models)})
}
func (h *Handler) SyncPreview(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		fail(c, err)
		return
	}
	if h.catalog == nil {
		fail(c, errors.New("AI catalogue source unavailable"))
		return
	}
	out, err := h.catalog.Preview(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, out)
}
func (h *Handler) SyncConfirm(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		Digest string             `json:"digest"`
		Models []CatalogSelection `json:"models"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if h.catalog == nil {
		fail(c, errors.New("AI catalogue source unavailable"))
		return
	}
	if err := h.catalog.Confirm(c.Request.Context(), id, req.Digest, req.Models); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) ModelList(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	if _, err := h.service.Detail(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	p, s, err := pagination(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := h.models.List(c.Request.Context(), id, p, s)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{"list": rows, "total": total, "page": p, "pageSize": s})
}
func (h *Handler) ModelSave(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		Models []price.SaveModelRequest `json:"models"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if _, err := h.service.Detail(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	if err := h.models.Save(c.Request.Context(), id, req.Models); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) ModelStatus(c *gin.Context) {
	providerID, modelID, err := h.ownedModel(c)
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if err := h.models.SetStatus(c.Request.Context(), providerID, modelID, req.Status); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
func (h *Handler) ownedModel(c *gin.Context) (int64, int64, error) {
	providerID, err := idParam(c, "id")
	if err != nil {
		return 0, 0, err
	}
	modelID, err := idParam(c, "modelId")
	if err != nil {
		return 0, 0, err
	}
	if _, err := h.service.Detail(c.Request.Context(), providerID); err != nil {
		return 0, 0, err
	}
	if _, err := h.models.Find(c.Request.Context(), providerID, modelID); err != nil {
		return 0, 0, err
	}
	return providerID, modelID, nil
}

type PriceRateResponse struct {
	ID               string          `json:"id"`
	Operation        string          `json:"operation"`
	UsageKey         string          `json:"usageKey"`
	Selector         json.RawMessage `json:"selector"`
	Unit             string          `json:"unit"`
	UnitQuantity     string          `json:"unitQuantity"`
	SourceUnitPrice  string          `json:"sourceUnitPrice"`
	BillingUnitPrice *string         `json:"billingUnitPrice"`
}
type PriceVersionResponse struct {
	ID              string              `json:"id"`
	ProviderModelID string              `json:"providerModelId"`
	Version         string              `json:"version"`
	Status          int16               `json:"status"`
	EffectiveAt     *time.Time          `json:"effectiveAt"`
	Currency        string              `json:"currency"`
	SourceCurrency  string              `json:"sourceCurrency"`
	SourceDigest    string              `json:"sourceDigest"`
	SnapshotDigest  string              `json:"snapshotDigest"`
	SyncedAt        time.Time           `json:"syncedAt"`
	Rates           []PriceRateResponse `json:"rates"`
}

func priceResponse(p *price.PriceVersion) PriceVersionResponse {
	out := PriceVersionResponse{ID: strconv.FormatInt(p.ID, 10), ProviderModelID: strconv.FormatInt(p.ProviderModelID, 10), Version: strconv.FormatInt(p.Version, 10), Status: p.Status, EffectiveAt: p.EffectiveAt, Currency: p.Currency, SourceCurrency: p.SourceCurrency, SourceDigest: p.SourceDigest, SnapshotDigest: p.SnapshotDigest, SyncedAt: p.SyncedAt, Rates: []PriceRateResponse{}}
	for _, r := range p.Rates {
		var billing *string
		if r.BillingUnitPriceUnits != nil {
			v, _ := money.FormatRMBUnits(*r.BillingUnitPriceUnits)
			billing = &v
		}
		out.Rates = append(out.Rates, PriceRateResponse{strconv.FormatInt(r.ID, 10), r.Operation, r.UsageKey, r.Selector, r.Unit, r.UnitQuantity, r.SourceUnitPrice, billing})
	}
	return out
}
func (h *Handler) PriceList(c *gin.Context) {
	_, modelID, err := h.ownedModel(c)
	if err != nil {
		fail(c, err)
		return
	}
	p, s, err := pagination(c)
	if err != nil {
		fail(c, err)
		return
	}
	rows, total, err := h.prices.List(c.Request.Context(), modelID, p, s)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]PriceVersionResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, priceResponse(row))
	}
	response.OK(c, 200, map[string]any{"list": out, "total": total, "page": p, "pageSize": s})
}
func (h *Handler) PriceSync(c *gin.Context) {
	providerID, modelID, err := h.ownedModel(c)
	if err != nil {
		fail(c, err)
		return
	}
	if err := validate.RequireEmptyBody(c); err != nil {
		fail(c, err)
		return
	}
	if h.priceSync == nil {
		fail(c, errors.New("AI price source unavailable"))
		return
	}
	out, err := h.priceSync.Preview(c.Request.Context(), providerID, modelID, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 201, priceResponse(out))
}
func (h *Handler) PriceUpdate(c *gin.Context) {
	_, modelID, err := h.ownedModel(c)
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		VersionID   string     `json:"versionId"`
		EffectiveAt *time.Time `json:"effectiveAt"`
		Rates       []struct {
			RateID           string `json:"rateId"`
			BillingUnitPrice string `json:"billingUnitPrice"`
		} `json:"rates"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	versionID, err := validate.ParsePositiveInt64(req.VersionID, "versionId")
	if err != nil {
		fail(c, err)
		return
	}
	update := price.UpdatePriceRequest{EffectiveAt: req.EffectiveAt, Rates: []price.UpdateRate{}}
	for _, r := range req.Rates {
		id, err := validate.ParsePositiveInt64(r.RateID, "rateId")
		if err != nil {
			fail(c, err)
			return
		}
		units, err := money.ParseRMBUnits(r.BillingUnitPrice)
		if err != nil || units < 0 {
			fail(c, apperror.InvalidRequest(errors.New("RMB price invalid")))
			return
		}
		update.Rates = append(update.Rates, price.UpdateRate{RateID: id, BillingUnitPriceUnits: &units})
	}
	out, err := h.prices.PreviewUpdate(c.Request.Context(), modelID, versionID, update, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 201, priceResponse(out))
}
func (h *Handler) PricePublish(c *gin.Context) {
	_, modelID, err := h.ownedModel(c)
	if err != nil {
		fail(c, err)
		return
	}
	versionID, err := idParam(c, "versionId")
	if err != nil {
		fail(c, err)
		return
	}
	var req struct {
		SnapshotDigest string `json:"snapshotDigest"`
	}
	if err := validate.BindJSON(c, &req); err != nil {
		fail(c, err)
		return
	}
	if err := h.prices.Publish(c.Request.Context(), modelID, versionID, req.SnapshotDigest, actor(c)); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, 200, map[string]any{})
}
