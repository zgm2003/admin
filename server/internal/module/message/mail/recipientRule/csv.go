package recipientrule

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/i18n"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/objectKey"
	"github.com/jackc/pgx/v5/pgconn"
)

var csvHeader = []string{"类型", "邮箱/域名", "动作", "名称", "备注", "启用状态"}

func (s *Service) ImportTemplate(ctx context.Context) (ImportTemplate, error) {
	if s.settings == nil {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("settings reader unavailable"))
	}
	row, err := s.settings.FindByKey(ctx, sharedsetting.MailRecipientRuleImportTemplateObjectKey)
	if err != nil {
		return ImportTemplate{}, apperror.DependencyUnavailable(err)
	}
	if row.ValueType != sharedsetting.ValueTypeString || row.IsEnabled != yesno.Yes {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("recipient rule template setting invalid"))
	}
	value := strings.TrimSpace(row.Value)
	if value != "" && (objectkey.Validate(value) != nil || !strings.HasSuffix(value, ".csv")) {
		return ImportTemplate{}, apperror.DependencyUnavailable(fmt.Errorf("recipient rule template object key invalid"))
	}
	return ImportTemplate{ObjectKey: value}, nil
}

func (s *Service) PreviewCSV(ctx context.Context, content string) (CSVPreview, error) {
	preview := parseCSV(content)
	if len(preview.Errors) != 0 {
		return preview, nil
	}
	keys := make([][]string, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		if len(row.Errors) == 0 || (len(row.Errors) == 1 && row.Errors[0] == "duplicate_file") {
			keys = append(keys, []string{row.Values[0], row.Values[1], row.Values[2]})
		}
	}
	existing, err := s.repository.FindMatching(ctx, keys)
	if err != nil {
		return CSVPreview{}, wrapRepository(err)
	}
	seen := make(map[string]bool, len(existing))
	for _, row := range existing {
		seen[csvKey([]string{row.Scope, row.Pattern, row.Action})] = true
	}
	for index := range preview.Rows {
		row := &preview.Rows[index]
		if len(row.Values) == len(csvHeader) && seen[csvKey(row.Values)] {
			row.Errors = append(row.Errors, "duplicate_existing")
		}
	}
	return preview, nil
}

func (s *Service) ImportCSV(ctx context.Context, content string) (CSVImportResult, error) {
	preview, err := s.PreviewCSV(ctx, content)
	if err != nil {
		return CSVImportResult{}, err
	}
	if len(preview.Errors) != 0 {
		return CSVImportResult{}, csvInvalidRequest(i18n.KeyMailRuleImportInvalid, fmt.Errorf("CSV file invalid"))
	}
	duplicate := false
	invalid := false
	for _, row := range preview.Rows {
		for _, code := range row.Errors {
			if code == "duplicate_file" || code == "duplicate_existing" {
				duplicate = true
			} else {
				invalid = true
			}
		}
	}
	if invalid {
		return CSVImportResult{}, csvInvalidRequest(i18n.KeyMailRuleImportInvalid, fmt.Errorf("CSV rows invalid"))
	}
	if duplicate {
		return CSVImportResult{}, apperror.Conflict(i18n.KeyMailRuleImportConflict, nil, fmt.Errorf("CSV duplicate rules"))
	}
	if s.runtime == nil {
		return CSVImportResult{}, apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	now := time.Now().UTC()
	values := make([]Model, len(preview.Rows))
	for index, row := range preview.Rows {
		cells := row.Values
		values[index] = Model{Scope: cells[0], Pattern: cells[1], Action: cells[2], Name: cells[3], Remark: cells[4], IsEnabled: yesno.Value(cells[5][0] - '0'), CreatedAt: now, UpdatedAt: now}
	}
	err = s.runtime.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.CreateBatch(writeContext, values, expected, now)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return CSVImportResult{}, apperror.Conflict(i18n.KeyMailRuleImportConflict, nil, err)
	}
	if err != nil {
		return CSVImportResult{}, wrapRepository(err)
	}
	return CSVImportResult{Imported: len(values)}, nil
}

func (s *Service) ExportCSV(ctx context.Context) (CSVFile, error) {
	rows, err := s.repository.ListForExport(ctx, CSVMaxRows+1)
	if err != nil {
		return CSVFile{}, wrapRepository(err)
	}
	if len(rows) > CSVMaxRows {
		return CSVFile{}, csvInvalidRequest(i18n.KeyMailRuleExportLimit, fmt.Errorf("CSV row limit exceeded"))
	}
	var output strings.Builder
	output.WriteString("\ufeff")
	writer := csv.NewWriter(&output)
	if err := writer.Write(csvHeader); err != nil {
		return CSVFile{}, apperror.Internal(err)
	}
	for _, row := range rows {
		cells := []string{row.Scope, row.Pattern, row.Action, row.Name, row.Remark, strconv.Itoa(int(row.IsEnabled))}
		for index := range cells {
			cells[index] = escapeCSVCell(cells[index])
		}
		if err := writer.Write(cells); err != nil {
			return CSVFile{}, apperror.Internal(err)
		}
		// Flush to enforce the byte limit while writing, not after an unbounded buffer.
		writer.Flush()
		if writer.Error() != nil {
			return CSVFile{}, apperror.Internal(writer.Error())
		}
		if output.Len() > CSVMaxBytes {
			return CSVFile{}, csvInvalidRequest(i18n.KeyMailRuleExportLimit, fmt.Errorf("CSV byte limit exceeded"))
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		return CSVFile{}, apperror.Internal(writer.Error())
	}
	return CSVFile{FileName: "mail-recipient-rule.csv", Content: output.String()}, nil
}

func parseCSV(content string) CSVPreview {
	preview := CSVPreview{Rows: []CSVRow{}, Errors: []string{}}
	if len(content) > CSVMaxBytes {
		preview.Errors = append(preview.Errors, "too_large")
		return preview
	}
	if !utf8.ValidString(content) {
		preview.Errors = append(preview.Errors, "invalid_encoding")
		return preview
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\ufeff")))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		preview.Errors = append(preview.Errors, "empty")
		return preview
	}
	if err != nil {
		preview.Errors = append(preview.Errors, "invalid_csv")
		return preview
	}
	if !slices.Equal(header, csvHeader) {
		preview.Errors = append(preview.Errors, "invalid_header")
		return preview
	}
	seen := make(map[string][]int)
	for {
		cells, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			preview.Errors = append(preview.Errors, "invalid_csv")
			break
		}
		if len(preview.Rows) >= CSVMaxRows {
			preview.Errors = append(preview.Errors, "too_many_rows")
			break
		}
		line, _ := reader.FieldPos(0)
		for index := range cells {
			cells[index] = unescapeCSVCell(cells[index])
		}
		row := CSVRow{Line: line, Values: cells, Errors: []string{}}
		validateCSVRow(&row)
		if len(row.Errors) == 0 {
			seen[csvKey(row.Values)] = append(seen[csvKey(row.Values)], len(preview.Rows))
		}
		preview.Rows = append(preview.Rows, row)
	}
	for _, indexes := range seen {
		if len(indexes) > 1 {
			for _, index := range indexes {
				preview.Rows[index].Errors = append(preview.Rows[index].Errors, "duplicate_file")
			}
		}
	}
	if len(preview.Rows) == 0 && len(preview.Errors) == 0 {
		preview.Errors = append(preview.Errors, "empty")
	}
	return preview
}

func validateCSVRow(row *CSVRow) {
	if len(row.Values) != len(csvHeader) {
		row.Errors = append(row.Errors, "invalid_columns")
		return
	}
	cells := row.Values
	for _, value := range cells {
		if strings.ContainsRune(value, '\x00') {
			row.Errors = append(row.Errors, "invalid_character")
			break
		}
	}
	for _, index := range []int{0, 1, 2, 3, 5} {
		cells[index] = strings.TrimSpace(cells[index])
	}
	if cells[0] != ScopeEmail && cells[0] != ScopeDomain {
		row.Errors = append(row.Errors, "invalid_scope")
	} else if pattern, err := NormalizeRule(cells[0], cells[1]); err != nil {
		row.Errors = append(row.Errors, "invalid_pattern")
	} else {
		cells[1] = pattern
	}
	if cells[2] != ActionAllow && cells[2] != ActionDeny {
		row.Errors = append(row.Errors, "invalid_action")
	}
	if cells[3] == "" || utf8.RuneCountInString(cells[3]) > 128 {
		row.Errors = append(row.Errors, "invalid_name")
	}
	if utf8.RuneCountInString(cells[4]) > 512 {
		row.Errors = append(row.Errors, "invalid_remark")
	}
	if cells[5] != "0" && cells[5] != "1" {
		row.Errors = append(row.Errors, "invalid_status")
	}
}

func csvKey(cells []string) string { return strings.Join(cells[:3], "\x00") }

// Escape apostrophes too, so the import can remove exactly our export prefix.
func escapeCSVCell(value string) string {
	if strings.HasPrefix(value, "'") || formulaCSVCell(value) {
		return "'" + value
	}
	return value
}

func unescapeCSVCell(value string) string {
	if strings.HasPrefix(value, "'") && (strings.HasPrefix(value[1:], "'") || formulaCSVCell(value[1:])) {
		return value[1:]
	}
	return value
}

func formulaCSVCell(value string) bool {
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return true
	}
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	return len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))
}

func csvInvalidRequest(key i18n.MessageKey, cause error) error {
	err := apperror.InvalidRequest(cause)
	err.MessageKey = key
	return err
}
