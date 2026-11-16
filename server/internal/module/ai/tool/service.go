package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	systemserver "admin/server/internal/module/system/server"

	"gorm.io/gorm"
)

const (
	CodeUserCount      = "system.user_count"
	CodeServerOverview = "system.server_overview"
)

type Invocation struct {
	PlatformID int64
	UserID     int64
	RunID      int64
}

type Definition struct {
	Code        string
	APIName     string
	Name        string
	Description string
	Category    string
	Parameters  map[string]any
	Execute     func(context.Context, json.RawMessage, Invocation) (any, error)
}

type Value struct {
	ID          int64          `json:"id"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Status      Status         `json:"status"`
	StatusName  string         `json:"statusName"`
	SortOrder   int32          `json:"sortOrder"`
	Parameters  map[string]any `json:"parameters"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type Snapshotter interface {
	Snapshot(context.Context) (systemserver.Snapshot, error)
}

type Service struct {
	repo      *Repository
	db        *gorm.DB
	snapshots Snapshotter
	logger    *slog.Logger
	defs      map[string]Definition
	apiNames  map[string]string
	now       func() time.Time
}

func NewService(repo *Repository, db *gorm.DB, snapshots Snapshotter, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	service := &Service{
		repo: repo, db: db, snapshots: snapshots, logger: logger, now: time.Now,
		apiNames: make(map[string]string, 2),
	}
	service.defs = map[string]Definition{
		CodeUserCount:      service.userCountDefinition(),
		CodeServerOverview: service.serverOverviewDefinition(),
	}
	for code, definition := range service.defs {
		service.apiNames[definition.APIName] = code
	}
	return service
}

func (s *Service) userCountDefinition() Definition {
	return Definition{
		Code: CodeUserCount, APIName: "system_user_count", Name: "用户总数", Category: "internal",
		Description: "查询当前系统中的用户总数、启用用户数、停用用户数和今日新增用户数。",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{},
		},
		Execute: s.executeUserCount,
	}
}

func (s *Service) serverOverviewDefinition() Definition {
	return Definition{
		Code: CodeServerOverview, APIName: "system_server_overview", Name: "服务器概览", Category: "internal",
		Description: "查询当前 API 服务器的操作系统、CPU、内存、磁盘、运行时长、Go 进程和数据库连接池概览。",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{},
		},
		Execute: s.executeServerOverview,
	}
}

func (s *Service) List(ctx context.Context) ([]Value, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Value, 0, len(rows))
	for _, row := range rows {
		definition, ok := s.defs[row.Code]
		if !ok {
			continue
		}
		result = append(result, Value{
			ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description,
			Category: row.Category, Status: row.Status, StatusName: row.Status.Label(),
			SortOrder: row.SortOrder, Parameters: definition.Parameters, UpdatedAt: row.UpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) Enabled(ctx context.Context) ([]Definition, error) {
	rows, err := s.repo.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Definition, 0, len(rows))
	for _, row := range rows {
		if definition, ok := s.defs[row.Code]; ok {
			result = append(result, definition)
		}
	}
	return result, nil
}

func (s *Service) UpdateStatus(ctx context.Context, code string, status Status) error {
	if !status.Valid() {
		return errors.New("tool status is invalid")
	}
	affected, err := s.repo.UpdateStatus(ctx, code, status, s.now().UTC())
	if err != nil {
		return err
	}
	if affected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) Execute(ctx context.Context, code, arguments string, meta Invocation) (string, error) {
	definition, ok := s.defs[code]
	if !ok {
		if resolved, exists := s.apiNames[code]; exists {
			definition, ok = s.defs[resolved]
		}
	}
	if !ok {
		return "", fmt.Errorf("unknown tool %q", code)
	}
	if arguments == "" {
		arguments = "{}"
	}
	var params json.RawMessage = []byte(arguments)
	if !json.Valid(params) {
		return "", fmt.Errorf("tool arguments are not valid JSON")
	}
	value, err := definition.Execute(ctx, params, meta)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if len(payload) > 12000 {
		payload = payload[:12000]
	}
	return string(payload), nil
}

func (s *Service) executeUserCount(ctx context.Context, _ json.RawMessage, _ Invocation) (any, error) {
	if s.db == nil {
		return nil, errors.New("database is not configured")
	}
	startOfDay := s.now().UTC().Truncate(24 * time.Hour)
	var row struct {
		Total    int64 `gorm:"column:total"`
		Active   int64 `gorm:"column:active"`
		Disabled int64 `gorm:"column:disabled"`
		NewToday int64 `gorm:"column:new_today"`
	}
	err := s.db.WithContext(ctx).Table("user_account").
		Select(`count(*) AS total,
			count(*) FILTER (WHERE status = 1) AS active,
			count(*) FILTER (WHERE status = 0) AS disabled,
			count(*) FILTER (WHERE created_at >= ?) AS new_today`, startOfDay).
		Where("deleted_at IS NULL").Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total": row.Total, "active": row.Active, "disabled": row.Disabled,
		"newToday": row.NewToday, "sampledAt": s.now().UTC(),
	}, nil
}

type databaseOverview struct {
	OpenConnections int    `json:"openConnections"`
	InUse           int    `json:"inUse"`
	Idle            int    `json:"idle"`
	WaitCount       int64  `json:"waitCount"`
	WaitDuration    string `json:"waitDuration"`
}

type serverOverview struct {
	systemserver.Snapshot
	Database databaseOverview `json:"database"`
}

func (s *Service) executeServerOverview(ctx context.Context, _ json.RawMessage, _ Invocation) (any, error) {
	if s.snapshots == nil {
		return nil, errors.New("server snapshot is not configured")
	}
	snapshot, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	result := serverOverview{Snapshot: snapshot}
	if s.db != nil {
		if sqlDB, dbErr := s.db.DB(); dbErr == nil {
			stats := sqlDB.Stats()
			result.Database = databaseOverview{
				OpenConnections: stats.OpenConnections, InUse: stats.InUse, Idle: stats.Idle,
				WaitCount: stats.WaitCount, WaitDuration: stats.WaitDuration.String(),
			}
		}
	}
	return result, nil
}
