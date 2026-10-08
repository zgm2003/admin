package recipientrule

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/i18n"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/objectKey"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) ImportTemplate(ctx context.Context) (ImportTemplate, error) {
	if s.settings == nil {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("settings reader unavailable"))
	}
	row, err := s.settings.FindByKey(ctx, sharedsetting.MailRecipientRuleImportTemplateObjectKey)
	if err != nil {
		return ImportTemplate{}, apperror.DependencyUnavailable(err)
	}
	if row.ValueType != sharedsetting.ValueTypeMedia || row.IsEnabled != yesno.Yes {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("recipient rule template setting invalid"))
	}
	value := strings.TrimSpace(row.Value)
	if value != "" && (objectkey.Validate(value) != nil || !strings.HasSuffix(value, ".xlsx")) {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("recipient rule template object key invalid"))
	}
	return ImportTemplate{ObjectKey: value}, nil
}

func (s *Service) PreviewXlsx(ctx context.Context, input XlsxImportInput) (XlsxPreview, error) {
	content, err := decodeXlsxImportInput(input)
	if err != nil {
		return XlsxPreview{}, apperror.InvalidRequest(err)
	}
	preview := parseXlsx(ctx, content)
	if err := ctx.Err(); err != nil {
		return XlsxPreview{}, apperror.DependencyUnavailable(err)
	}
	if len(preview.Errors) != 0 {
		return preview, nil
	}
	keys := make([]ruleKey, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		if row.Data != nil {
			keys = append(keys, xlsxRuleKey(*row.Data))
		}
	}
	existing, err := s.repository.FindMatching(ctx, keys)
	if err != nil {
		return XlsxPreview{}, wrapRepository(err)
	}
	seen := make(map[ruleKey]bool, len(existing))
	for _, row := range existing {
		if !row.Scope.IsValid() || !row.Action.IsValid() {
			return XlsxPreview{}, wrapRepository(fmt.Errorf("stored recipient rule enum invalid"))
		}
		seen[ruleKey{Scope: row.Scope, Pattern: row.Pattern, Action: row.Action}] = true
	}
	for index := range preview.Rows {
		row := &preview.Rows[index]
		if row.Data != nil && seen[xlsxRuleKey(*row.Data)] {
			row.Errors = append(row.Errors, "duplicate_existing")
		}
	}
	return preview, nil
}

func (s *Service) ImportXlsx(ctx context.Context, input XlsxImportInput) (XlsxImportResult, error) {
	preview, err := s.PreviewXlsx(ctx, input)
	if err != nil {
		return XlsxImportResult{}, err
	}
	if len(preview.Errors) != 0 {
		return XlsxImportResult{}, xlsxInvalidRequest(i18n.KeyMailRuleImportInvalid, fmt.Errorf("Xlsx file invalid"))
	}
	duplicate := false
	invalid := false
	for _, row := range preview.Rows {
		if row.Data == nil {
			invalid = true
		}
		for _, code := range row.Errors {
			if code == "duplicate_file" || code == "duplicate_existing" {
				duplicate = true
			} else {
				invalid = true
			}
		}
	}
	if invalid {
		return XlsxImportResult{}, xlsxInvalidRequest(i18n.KeyMailRuleImportInvalid, fmt.Errorf("Xlsx rows invalid"))
	}
	if duplicate {
		return XlsxImportResult{}, apperror.Conflict(i18n.KeyMailRuleImportConflict, nil, fmt.Errorf("Xlsx duplicate rules"))
	}
	if s.runtime == nil {
		return XlsxImportResult{}, apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	now := time.Now().UTC()
	values := make([]Model, len(preview.Rows))
	for index, row := range preview.Rows {
		data := *row.Data
		values[index] = Model{Scope: data.Scope, Pattern: data.Pattern, Action: data.Action, Name: data.Name, Remark: data.Remark, IsEnabled: data.IsEnabled, CreatedAt: now, UpdatedAt: now}
	}
	err = s.runtime.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.CreateBatch(writeContext, values, expected, now)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return XlsxImportResult{}, apperror.Conflict(i18n.KeyMailRuleImportConflict, nil, err)
	}
	if err != nil {
		return XlsxImportResult{}, wrapRepository(err)
	}
	return XlsxImportResult{Imported: len(values)}, nil
}

func (s *Service) ExportXlsx(ctx context.Context) (XlsxExportFile, error) {
	rows, err := s.repository.ListForExport(ctx, XlsxMaxRows+1)
	if err != nil {
		return XlsxExportFile{}, wrapRepository(err)
	}
	if len(rows) > XlsxMaxRows {
		return XlsxExportFile{}, xlsxInvalidRequest(i18n.KeyMailRuleExportLimit, fmt.Errorf("workbook row limit exceeded"))
	}
	value, err := encodeXlsx(ctx, rows)
	if err != nil {
		return XlsxExportFile{}, apperror.Internal(err)
	}
	return value, nil
}
func validateXlsxRow(row *XlsxRow) {
	row.Data = nil
	if len(row.RawValues) != len(xlsxHeader) {
		row.Errors = append(row.Errors, "invalid_columns")
		return
	}
	// Preserve the source text for diagnostics; normalization belongs only to Data.
	cells := slices.Clone(row.RawValues)
	for _, value := range cells {
		if !validXlsxText(value) {
			row.Errors = append(row.Errors, "invalid_character")
			break
		}
	}
	for _, index := range []int{0, 1, 2, 3, 5} {
		cells[index] = strings.TrimSpace(cells[index])
	}
	scope, err := parseXlsxScope(cells[0])
	if err != nil {
		row.Errors = append(row.Errors, "invalid_scope")
	} else if pattern, err := NormalizeRule(scope, cells[1]); err != nil {
		row.Errors = append(row.Errors, "invalid_pattern")
	} else {
		cells[1] = pattern
	}
	action, err := parseXlsxAction(cells[2])
	if err != nil {
		row.Errors = append(row.Errors, "invalid_action")
	}
	if cells[3] == "" || utf8.RuneCountInString(cells[3]) > 128 {
		row.Errors = append(row.Errors, "invalid_name")
	}
	if utf8.RuneCountInString(cells[4]) > 512 {
		row.Errors = append(row.Errors, "invalid_remark")
	}
	if cells[5] != "停用" && cells[5] != "启用" {
		row.Errors = append(row.Errors, "invalid_status")
	}
	if len(row.Errors) == 0 {
		enabled := yesno.No
		if cells[5] == "启用" {
			enabled = yesno.Yes
		}
		row.Data = &Input{Scope: scope, Pattern: cells[1], Action: action, Name: cells[3], Remark: cells[4], IsEnabled: enabled}
	}
}

func xlsxRuleKey(data Input) ruleKey {
	return ruleKey{Scope: data.Scope, Pattern: data.Pattern, Action: data.Action}
}

func xlsxInvalidRequest(key i18n.MessageKey, cause error) error {
	err := apperror.InvalidRequest(cause)
	err.MessageKey = key
	return err
}

func parseXlsxScope(value string) (Scope, error) {
	switch value {
	case "邮箱":
		return ScopeEmail, nil
	case "域名":
		return ScopeDomain, nil
	}
	return 0, fmt.Errorf("invalid Xlsx scope")
}
func parseXlsxAction(value string) (Action, error) {
	switch value {
	case "拒绝":
		return ActionDeny, nil
	case "允许":
		return ActionAllow, nil
	}
	return 0, fmt.Errorf("invalid Xlsx action")
}
func formatXlsxScope(value Scope) (string, error) {
	switch value {
	case ScopeEmail:
		return "邮箱", nil
	case ScopeDomain:
		return "域名", nil
	}
	return "", fmt.Errorf("invalid recipient rule scope")
}
func formatXlsxAction(value Action) (string, error) {
	switch value {
	case ActionDeny:
		return "拒绝", nil
	case ActionAllow:
		return "允许", nil
	}
	return "", fmt.Errorf("invalid recipient rule action")
}
