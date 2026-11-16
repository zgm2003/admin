package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	systemserver "admin/server/internal/module/system/server"

	"gorm.io/gorm"
)

const (
	CodeUserCount      = "system.user_count"
	CodeServerOverview = "system.server_overview"

	defaultTimeoutMS = int32(10000)
	minTimeoutMS     = int32(1000)
	maxTimeoutMS     = int32(60000)
	maxParameters    = 20 * 1024
	maxURLRunes      = 2000
	maxDescription   = 500
	maxName          = 128
	maxCategory      = 32
	maxHTTPBody      = 64 * 1024
)

var (
	codePattern     = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	apiNameReplacer = strings.NewReplacer(".", "_", "-", "_")
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
	APIName     string         `json:"apiName"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Kind        Kind           `json:"kind"`
	KindName    string         `json:"kindName"`
	Status      Status         `json:"status"`
	StatusName  string         `json:"statusName"`
	SortOrder   int32          `json:"sortOrder"`
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	TimeoutMS   int32          `json:"timeoutMs"`
	Parameters  map[string]any `json:"parameters"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type CreateInput struct {
	Code        string
	Name        string
	Description string
	Category    string
	Parameters  json.RawMessage
	Method      string
	URL         string
	TimeoutMS   int32
	Status      Status
	SortOrder   int32
}

type UpdateInput struct {
	Name        string
	Description string
	Category    string
	Parameters  json.RawMessage
	Method      string
	URL         string
	TimeoutMS   int32
	Status      Status
	SortOrder   int32
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
	now       func() time.Time
}

func NewService(repo *Repository, db *gorm.DB, snapshots Snapshotter, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	service := &Service{
		repo: repo, db: db, snapshots: snapshots, logger: logger, now: time.Now,
	}
	service.defs = map[string]Definition{
		CodeUserCount:      service.userCountDefinition(),
		CodeServerOverview: service.serverOverviewDefinition(),
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
		parameters, err := parametersFromRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, valueFromRow(row, parameters))
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, code string) (Value, error) {
	row, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return Value{}, err
	}
	parameters, err := parametersFromRow(row)
	if err != nil {
		return Value{}, err
	}
	return valueFromRow(row, parameters), nil
}

func (s *Service) Enabled(ctx context.Context) ([]Definition, error) {
	rows, err := s.repo.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	return s.definitions(rows)
}

func (s *Service) EnabledForAgent(ctx context.Context, agentID int64) ([]Definition, error) {
	if agentID < 1 {
		return nil, errors.New("agent id is invalid")
	}
	rows, err := s.repo.EnabledForAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return s.definitions(rows)
}

func (s *Service) definitions(rows []Tool) ([]Definition, error) {
	result := make([]Definition, 0, len(rows))
	for _, row := range rows {
		definition, err := s.definition(row)
		if err != nil {
			return nil, err
		}
		result = append(result, definition)
	}
	return result, nil
}

func (s *Service) definition(row Tool) (Definition, error) {
	parameters, err := parametersFromRow(row)
	if err != nil {
		return Definition{}, err
	}
	apiName := strings.TrimSpace(row.APIName)
	if apiName == "" {
		apiName = apiNameFor(row.Code)
	}
	definition := Definition{
		Code: row.Code, APIName: apiName, Name: row.Name, Description: row.Description,
		Category: row.Category, Parameters: parameters,
	}
	switch row.Kind {
	case KindBuiltin:
		handler, ok := s.defs[row.Code]
		if !ok || handler.Execute == nil {
			return Definition{}, fmt.Errorf("built-in tool handler is missing for %q", row.Code)
		}
		definition.Execute = handler.Execute
	case KindHTTP:
		executor, err := s.httpExecutor(row)
		if err != nil {
			return Definition{}, err
		}
		definition.Execute = executor
	default:
		return Definition{}, fmt.Errorf("tool kind is invalid for %q", row.Code)
	}
	return definition, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, dependency(errors.New("tool repository is not configured"))
	}
	code := strings.TrimSpace(input.Code)
	if !codePattern.MatchString(code) {
		return 0, invalid(errors.New("code must start with a lowercase letter and contain only lowercase letters, numbers, _, . or -"))
	}
	if _, err := s.repo.FindByCode(ctx, code); err == nil {
		return 0, invalid(fmt.Errorf("code %q already exists", code))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, mapError(err)
	}
	values, err := normalizeInput(input.Name, input.Description, input.Category, input.Parameters,
		input.Method, input.URL, input.TimeoutMS, input.Status, input.SortOrder, true)
	if err != nil {
		return 0, invalid(err)
	}
	apiName := apiNameFor(code)
	if existing, err := s.repo.FindByIdentifier(ctx, apiName); err == nil {
		return 0, invalid(fmt.Errorf("api name %q conflicts with tool %q", apiName, existing.Code))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, mapError(err)
	}
	now := s.now().UTC()
	id, err := s.repo.Create(ctx, Tool{
		Code: code, APIName: apiName, Name: values.Name, Description: values.Description,
		Category: values.Category, Kind: KindHTTP, Parameters: values.Parameters,
		Method: values.Method, URL: values.URL, TimeoutMS: values.TimeoutMS,
		Status: values.Status, SortOrder: values.SortOrder, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return 0, mapError(err)
	}
	return id, nil
}

func (s *Service) Update(ctx context.Context, code string, input UpdateInput) error {
	if s == nil || s.repo == nil {
		return dependency(errors.New("tool repository is not configured"))
	}
	row, err := s.repo.FindByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return mapError(err)
	}
	values, err := normalizeInput(input.Name, input.Description, input.Category, input.Parameters,
		input.Method, input.URL, input.TimeoutMS, input.Status, input.SortOrder, row.Kind == KindHTTP)
	if err != nil {
		return invalid(err)
	}
	if row.Kind == KindBuiltin {
		values.Parameters = row.Parameters
		values.Method = row.Method
		values.URL = row.URL
		values.TimeoutMS = row.TimeoutMS
	}
	affected, err := s.repo.Update(ctx, row.Code, UpdateValues{
		APIName: row.APIName, Name: values.Name, Description: values.Description,
		Category: values.Category, Parameters: values.Parameters, Method: values.Method,
		URL: values.URL, TimeoutMS: values.TimeoutMS, Status: values.Status,
		SortOrder: values.SortOrder, UpdatedAt: s.now().UTC(),
	})
	if err != nil {
		return mapError(err)
	}
	if affected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, code string, status Status) error {
	if !status.Valid() {
		return invalid(errors.New("tool status is invalid"))
	}
	affected, err := s.repo.UpdateStatus(ctx, code, status, s.now().UTC())
	if err != nil {
		return mapError(err)
	}
	if affected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, code string) error {
	row, err := s.repo.FindByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return mapError(err)
	}
	if row.Kind == KindBuiltin {
		return invalid(errors.New("built-in tools cannot be deleted"))
	}
	affected, err := s.repo.Delete(ctx, row.Code, s.now().UTC())
	if err != nil {
		return mapError(err)
	}
	if affected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) Execute(ctx context.Context, identifier, arguments string, meta Invocation) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("tool repository is not configured")
	}
	row, err := s.repo.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("unknown tool %q", identifier)
		}
		return "", err
	}
	if row.Status != StatusEnabled {
		return "", fmt.Errorf("tool %q is disabled", row.Code)
	}
	definition, err := s.definition(row)
	if err != nil {
		return "", err
	}
	if arguments == "" {
		arguments = "{}"
	}
	if !json.Valid([]byte(arguments)) {
		return "", fmt.Errorf("tool arguments are not valid JSON")
	}
	value, err := definition.Execute(ctx, json.RawMessage(arguments), meta)
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

type normalizedInput struct {
	Name        string
	Description string
	Category    string
	Parameters  string
	Method      string
	URL         string
	TimeoutMS   int32
	Status      Status
	SortOrder   int32
}

func normalizeInput(name, description, category string, rawParameters json.RawMessage,
	method, rawURL string, timeoutMS int32, status Status, sortOrder int32, requireEndpoint bool,
) (normalizedInput, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	category = strings.TrimSpace(category)
	method = strings.ToUpper(strings.TrimSpace(method))
	rawURL = strings.TrimSpace(rawURL)
	if name == "" || len([]rune(name)) > maxName {
		return normalizedInput{}, fmt.Errorf("name must be between 1 and %d characters", maxName)
	}
	if len([]rune(description)) > maxDescription {
		return normalizedInput{}, fmt.Errorf("description must be at most %d characters", maxDescription)
	}
	if category == "" {
		category = "internal"
	}
	if len([]rune(category)) > maxCategory {
		return normalizedInput{}, fmt.Errorf("category must be at most %d characters", maxCategory)
	}
	if sortOrder < 0 || sortOrder > 9999 {
		return normalizedInput{}, errors.New("sortOrder must be between 0 and 9999")
	}
	if !status.Valid() {
		return normalizedInput{}, errors.New("status is invalid")
	}
	if timeoutMS == 0 {
		timeoutMS = defaultTimeoutMS
	}
	if timeoutMS < minTimeoutMS || timeoutMS > maxTimeoutMS {
		return normalizedInput{}, fmt.Errorf("timeoutMs must be between %d and %d", minTimeoutMS, maxTimeoutMS)
	}
	parameters, err := normalizeParameters(rawParameters)
	if err != nil {
		return normalizedInput{}, err
	}
	if requireEndpoint {
		if !validHTTPMethod(method) {
			return normalizedInput{}, errors.New("method must be GET, POST, PUT, PATCH or DELETE")
		}
		if len([]rune(rawURL)) > maxURLRunes || !validToolURL(rawURL) {
			return normalizedInput{}, errors.New("url must be a valid http or https URL")
		}
	}
	return normalizedInput{
		Name: name, Description: description, Category: category, Parameters: parameters,
		Method: method, URL: rawURL, TimeoutMS: timeoutMS, Status: status, SortOrder: sortOrder,
	}, nil
}

func normalizeParameters(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	if len(raw) > maxParameters {
		return "", errors.New("parameters is too large")
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", errors.New("parameters must be valid JSON")
	}
	if value == nil {
		return "", errors.New("parameters must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", errors.New("parameters must contain a single JSON value")
	}
	typeName, ok := value["type"].(string)
	if !ok || typeName != "object" {
		return "", errors.New(`parameters.type must be "object"`)
	}
	if _, ok := value["properties"].(map[string]any); !ok {
		return "", errors.New("parameters.properties must be a JSON object")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("parameters must be valid JSON")
	}
	return string(payload), nil
}

func parametersFromRow(row Tool) (map[string]any, error) {
	raw := strings.TrimSpace(row.Parameters)
	if raw == "" {
		raw = `{"type":"object","properties":{}}`
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil || value == nil {
		return nil, fmt.Errorf("tool %q has invalid parameters", row.Code)
	}
	return value, nil
}

func valueFromRow(row Tool, parameters map[string]any) Value {
	return Value{
		ID: row.ID, Code: row.Code, APIName: row.APIName, Name: row.Name,
		Description: row.Description, Category: row.Category, Kind: row.Kind,
		KindName: row.Kind.Label(), Status: row.Status, StatusName: row.Status.Label(),
		SortOrder: row.SortOrder, Method: row.Method, URL: row.URL, TimeoutMS: row.TimeoutMS,
		Parameters: parameters, UpdatedAt: row.UpdatedAt,
	}
}

func apiNameFor(code string) string {
	return apiNameReplacer.Replace(code)
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validToolURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (s *Service) httpExecutor(row Tool) (func(context.Context, json.RawMessage, Invocation) (any, error), error) {
	if !validHTTPMethod(row.Method) || !validToolURL(row.URL) {
		return nil, fmt.Errorf("tool %q has invalid HTTP configuration", row.Code)
	}
	method := row.Method
	endpoint := row.URL
	timeout := time.Duration(row.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(defaultTimeoutMS) * time.Millisecond
	}
	return func(ctx context.Context, raw json.RawMessage, _ Invocation) (any, error) {
		arguments := make(map[string]any)
		if len(bytes.TrimSpace(raw)) > 0 {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&arguments); err != nil {
				return nil, errors.New("tool arguments must be a JSON object")
			}
			if arguments == nil {
				arguments = make(map[string]any)
			}
		}
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		var body io.Reader
		if method == http.MethodGet || method == http.MethodDelete {
			query := parsed.Query()
			for key, value := range arguments {
				appendQueryValue(query, key, value)
			}
			parsed.RawQuery = query.Encode()
		} else {
			payload, err := json.Marshal(arguments)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(payload)
		}
		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		request, err := http.NewRequestWithContext(requestCtx, method, parsed.String(), body)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/json")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPBody+1))
		if err != nil {
			return nil, err
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			message := strings.TrimSpace(string(data))
			if len([]rune(message)) > 300 {
				message = string([]rune(message)[:300])
			}
			return nil, fmt.Errorf("tool endpoint returned %d: %s", response.StatusCode, message)
		}
		if len(data) > maxHTTPBody {
			data = data[:maxHTTPBody]
		}
		var decoded any
		if json.Unmarshal(data, &decoded) == nil {
			return decoded, nil
		}
		return string(data), nil
	}, nil
}

func appendQueryValue(query url.Values, key string, value any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			query.Add(key, fmt.Sprint(item))
		}
	case map[string]any:
		payload, _ := json.Marshal(typed)
		query.Add(key, string(payload))
	default:
		query.Add(key, fmt.Sprint(value))
	}
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
