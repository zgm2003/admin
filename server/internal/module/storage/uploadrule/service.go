package uploadrule

import (
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Service struct{ repository *Repository }

func NewService(r *Repository) *Service { return &Service{r} }
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.Result[RuleValue], error) {
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 {
		return pagination.Result[RuleValue]{}, invalid(fmt.Errorf("pagination invalid"))
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	n, e := s.repository.Count(ctx, q)
	if e != nil {
		return pagination.Result[RuleValue]{}, dependency(e)
	}
	rows, e := s.repository.List(ctx, q)
	if e != nil {
		return pagination.Result[RuleValue]{}, dependency(e)
	}
	return pagination.Result[RuleValue]{List: rows, Total: n, Page: q.Page, PageSize: q.PageSize}, nil
}
func (s *Service) PageInit(ctx context.Context) (PageInit, error) {
	p, e := s.repository.FindPlatformOptions(ctx)
	if e != nil {
		return PageInit{}, dependency(e)
	}
	c, e := s.repository.FindConfigSummaries(ctx)
	if e != nil {
		return PageInit{}, dependency(e)
	}
	return PageInit{p, c}, nil
}
func (s *Service) Get(ctx context.Context, id int64) (RuleValue, error) {
	if id < 1 {
		return RuleValue{}, invalid(fmt.Errorf("id invalid"))
	}
	m, e := s.repository.FindByID(ctx, id)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return RuleValue{}, notFound(e)
	}
	if e != nil {
		return RuleValue{}, dependency(e)
	}
	return RuleValue{ID: m.ID, PlatformID: m.PlatformID, Code: m.Code, Name: m.Name, CosConfigID: m.CosConfigID, PathPrefix: m.PathPrefix, MaxFileSizeBytes: m.MaxFileSizeBytes, MaxFileCount: m.MaxFileCount, AllowedExtensions: []string(m.AllowedExtensions), AllowedMimeTypes: []string(m.AllowedMimeTypes), AccessMode: m.AccessMode, IsEnabled: m.IsEnabled, Remark: m.Remark, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (int64, error) {
	in = normalizeCreateInput(in)
	if e := validateFields(in.PlatformID, in.Code, in.Name, in.CosConfigID, in.PathPrefix, in.MaxFileSizeBytes, in.MaxFileCount, in.AllowedExtensions, in.AllowedMimeTypes, in.AccessMode, in.Remark, true); e != nil {
		return 0, invalid(e)
	}
	platformOK, err := s.repository.PlatformEnabled(ctx, in.PlatformID)
	if err != nil {
		return 0, dependency(err)
	}
	if !platformOK {
		return 0, conflict(fmt.Errorf("platform unavailable"))
	}
	config, err := s.repository.Config(ctx, in.CosConfigID)
	if err != nil {
		return 0, conflict(fmt.Errorf("COS config unavailable"))
	}
	if in.AccessMode == "public" && (config.BucketDomain == nil || strings.TrimSpace(*config.BucketDomain) == "") {
		return 0, conflict(fmt.Errorf("public rule requires bucket domain"))
	}
	return s.repositoryCreate(ctx, in)
}
func (s *Service) repositoryCreate(ctx context.Context, in CreateInput) (int64, error) {
	m := &Model{PlatformID: in.PlatformID, Code: in.Code, Name: in.Name, CosConfigID: in.CosConfigID, PathPrefix: in.PathPrefix, MaxFileSizeBytes: in.MaxFileSizeBytes, MaxFileCount: in.MaxFileCount, AllowedExtensions: StringArray(in.AllowedExtensions), AllowedMimeTypes: StringArray(in.AllowedMimeTypes), AccessMode: in.AccessMode, IsEnabled: in.IsEnabled, Remark: in.Remark, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if in.IsEnabled == yesno.Yes {
		if e := s.repository.Transaction(ctx, func(r *Repository) error {
			rows, e := r.LockActiveByPlatform(ctx, in.PlatformID)
			if e != nil {
				return e
			}
			for _, row := range rows {
				if row.IsEnabled == yesno.Yes {
					if e = r.Update(ctx, row.ID, map[string]any{"is_enabled": yesno.No, "updated_at": m.UpdatedAt}); e != nil {
						return e
					}
				}
			}
			return r.Create(ctx, m)
		}); e != nil {
			if errors.Is(e, ErrConflict) {
				return 0, conflict(e)
			}
			return 0, dependency(e)
		}
	} else if e := s.repository.Create(ctx, m); e != nil {
		if errors.Is(e, ErrConflict) {
			return 0, conflict(e)
		}
		return 0, dependency(e)
	}
	return m.ID, nil
}
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	in = normalizeUpdateInput(in)
	if e := validateFields(1, "x", in.Name, in.CosConfigID, in.PathPrefix, in.MaxFileSizeBytes, in.MaxFileCount, in.AllowedExtensions, in.AllowedMimeTypes, in.AccessMode, in.Remark, false); e != nil {
		return invalid(e)
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		m, e := r.LockByID(ctx, id)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		platformOK, e := r.PlatformEnabled(ctx, m.PlatformID)
		if e != nil || !platformOK {
			return conflict(fmt.Errorf("platform unavailable"))
		}
		config, e := r.Config(ctx, in.CosConfigID)
		if e != nil {
			return conflict(fmt.Errorf("COS config unavailable"))
		}
		if in.AccessMode == "public" && (config.BucketDomain == nil || strings.TrimSpace(*config.BucketDomain) == "") {
			return conflict(fmt.Errorf("public rule requires bucket domain"))
		}
		if e = r.Update(ctx, id, map[string]any{"name": in.Name, "cos_config_id": in.CosConfigID, "path_prefix": in.PathPrefix, "max_file_size_bytes": in.MaxFileSizeBytes, "max_file_count": in.MaxFileCount, "allowed_extensions": StringArray(in.AllowedExtensions), "allowed_mime_types": StringArray(in.AllowedMimeTypes), "access_mode": in.AccessMode, "remark": in.Remark, "updated_at": time.Now().UTC()}); e != nil {
			return dependency(e)
		}
		_ = m
		return nil
	})
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, v yesno.Value) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if !yesno.IsValid(v) {
		return invalid(fmt.Errorf("isEnabled invalid"))
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		var m Model
		var e error
		if v == yesno.Yes {
			var rows []Model
			m, e = r.FindByID(ctx, id)
			if e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return notFound(e)
				}
				return dependency(e)
			}
			rows, e = r.LockActiveByPlatform(ctx, m.PlatformID)
			if e != nil {
				return dependency(e)
			}
			if _, e = r.Config(ctx, m.CosConfigID); e != nil {
				return conflict(fmt.Errorf("COS config unavailable"))
			}
			for _, row := range rows {
				if row.ID != id && row.IsEnabled == yesno.Yes {
					if e = r.Update(ctx, row.ID, map[string]any{"is_enabled": yesno.No, "updated_at": time.Now().UTC()}); e != nil {
						return dependency(e)
					}
				}
			}
		} else {
			m, e = r.LockByID(ctx, id)
			if e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return notFound(e)
				}
				return dependency(e)
			}
		}
		if e = r.Update(ctx, id, map[string]any{"is_enabled": v, "updated_at": time.Now().UTC()}); e != nil {
			if errors.Is(e, ErrConflict) {
				return conflict(e)
			}
			return dependency(e)
		}
		return nil
	})
}

func normalizeCreateInput(in CreateInput) CreateInput {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.PathPrefix = strings.Trim(strings.TrimSpace(in.PathPrefix), "/")
	in.Remark = strings.TrimSpace(in.Remark)
	in.AllowedExtensions = normalize(in.AllowedExtensions, true)
	in.AllowedMimeTypes = normalize(in.AllowedMimeTypes, false)
	return in
}
func normalizeUpdateInput(in UpdateInput) UpdateInput {
	in.Name = strings.TrimSpace(in.Name)
	in.PathPrefix = strings.Trim(strings.TrimSpace(in.PathPrefix), "/")
	in.Remark = strings.TrimSpace(in.Remark)
	in.AllowedExtensions = normalize(in.AllowedExtensions, true)
	in.AllowedMimeTypes = normalize(in.AllowedMimeTypes, false)
	return in
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	return s.repository.Transaction(ctx, func(r *Repository) error {
		m, e := r.LockByID(ctx, id)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		if m.IsEnabled == yesno.Yes {
			return conflict(fmt.Errorf("rule must be disabled"))
		}
		if e = r.MarkDeleted(ctx, id, time.Now().UTC()); e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return notFound(e)
			}
			return dependency(e)
		}
		return nil
	})
}
