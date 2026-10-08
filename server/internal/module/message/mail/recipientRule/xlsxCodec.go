package recipientrule

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const xlsxDataSheet = "导入数据"
const xlsxUnzipLimit = 16 << 20

var xlsxHeader = []string{"类型", "邮箱/域名", "动作", "名称", "备注", "启用状态"}

// encoding/json otherwise accepts case-insensitive struct field names. Keep
// this file transfer DTO exact; BindJSON also rejects duplicate keys/trailing data.
func (in *XlsxImportInput) UnmarshalJSON(raw []byte) error {
	var fields map[string]*string
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if len(fields) != 2 || fields["fileName"] == nil || fields["contentBase64"] == nil {
		return fmt.Errorf("fileName and contentBase64 must be present strings")
	}
	in.FileName, in.ContentBase64 = *fields["fileName"], *fields["contentBase64"]
	return nil
}

func decodeXlsxImportInput(in XlsxImportInput) ([]byte, error) {
	if len(in.FileName) <= len(".xlsx") || len(in.FileName) > 255 || !utf8.ValidString(in.FileName) ||
		strings.ContainsAny(in.FileName, "/\\:<>\"|?*") || strings.ContainsFunc(in.FileName, unicode.IsControl) || strings.TrimSpace(in.FileName) != in.FileName ||
		!strings.HasSuffix(strings.ToLower(in.FileName), ".xlsx") {
		return nil, fmt.Errorf("only an .xlsx file name is accepted")
	}
	stem := strings.ToUpper(strings.SplitN(in.FileName, ".", 2)[0])
	if slices.Contains([]string{"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$"}, stem) ||
		(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
		return nil, fmt.Errorf("file name is not a safe basename")
	}
	if len(in.ContentBase64) == 0 || len(in.ContentBase64) > base64.StdEncoding.EncodedLen(XlsxMaxBytes) ||
		strings.ContainsAny(in.ContentBase64, "\r\n\t ") {
		return nil, fmt.Errorf("workbook encoding is empty, malformed or too large")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
	if err != nil || len(content) == 0 || len(content) > XlsxMaxBytes {
		return nil, fmt.Errorf("workbook encoding is invalid")
	}
	return content, nil
}

func parseXlsx(ctx context.Context, content []byte) XlsxPreview {
	preview := XlsxPreview{Rows: []XlsxRow{}, Errors: []string{}}
	archive, code := inspectXlsxArchive(ctx, content)
	if code != "" {
		preview.Errors = append(preview.Errors, code)
		return preview
	}
	f, err := excelize.OpenReader(bytes.NewReader(content), excelize.Options{UnzipSizeLimit: xlsxUnzipLimit, UnzipXMLSizeLimit: xlsxUnzipLimit})
	if err != nil {
		preview.Errors = append(preview.Errors, "invalid_xlsx")
		return preview
	}
	defer f.Close()
	// GetRows ignores some per-cell errors and can return a truncated success.
	// Preflight has bounded this matrix; checked cell reads preserve all failures.
	rows := make([][]string, archive.dataRows)
	for r := range rows {
		if ctx.Err() != nil {
			preview.Errors = append(preview.Errors, "invalid_xlsx")
			return preview
		}
		rows[r] = make([]string, 6)
		for c := range rows[r] {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			rows[r][c], err = f.GetCellValue(xlsxDataSheet, cell, excelize.Options{RawCellValue: true})
			if err != nil {
				preview.Errors = append(preview.Errors, "invalid_xlsx")
				return preview
			}
		}
	}
	if len(rows) == 0 || !slices.Equal(rows[0], xlsxHeader) {
		preview.Errors = append(preview.Errors, "invalid_header")
		return preview
	}
	if len(rows) > XlsxMaxRows+1 {
		preview.Errors = append(preview.Errors, "too_many_rows")
		return preview
	}
	seen := map[ruleKey][]int{}
	for i, cells := range rows[1:] {
		if ctx.Err() != nil {
			preview.Errors = append(preview.Errors, "invalid_xlsx")
			return preview
		}
		empty := true
		for _, v := range cells {
			if strings.TrimSpace(v) != "" {
				empty = false
			}
		}
		if empty {
			continue
		}
		for len(cells) < 6 {
			cells = append(cells, "")
		}
		row := XlsxRow{Line: i + 2, RawValues: cells, Errors: []string{}}
		validateXlsxRow(&row)
		if row.Data != nil {
			key := xlsxRuleKey(*row.Data)
			seen[key] = append(seen[key], len(preview.Rows))
		}
		preview.Rows = append(preview.Rows, row)
	}
	for _, indexes := range seen {
		if len(indexes) > 1 {
			for _, i := range indexes {
				preview.Rows[i].Errors = append(preview.Rows[i].Errors, "duplicate_file")
			}
		}
	}
	if len(preview.Rows) == 0 {
		preview.Errors = append(preview.Errors, "empty")
	}
	return preview
}

func encodeXlsx(ctx context.Context, rows []Model) (XlsxExportFile, error) {
	if len(rows) > XlsxMaxRows {
		return XlsxExportFile{}, fmt.Errorf("XLSX row limit exceeded")
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", xlsxDataSheet); err != nil {
		return XlsxExportFile{}, err
	}
	values := make([][]string, 0, len(rows)+1)
	values = append(values, xlsxHeader)
	for _, r := range rows {
		scope, se := formatXlsxScope(r.Scope)
		action, ae := formatXlsxAction(r.Action)
		if se != nil || ae != nil || (r.IsEnabled != 0 && r.IsEnabled != 1) {
			return XlsxExportFile{}, fmt.Errorf("invalid stored rule enums")
		}
		status := "停用"
		if r.IsEnabled == 1 {
			status = "启用"
		}
		cells := []string{scope, r.Pattern, action, r.Name, r.Remark, status}
		row := XlsxRow{RawValues: cells}
		validateXlsxRow(&row)
		if len(row.Errors) != 0 {
			return XlsxExportFile{}, fmt.Errorf("invalid stored recipient rule")
		}
		values = append(values, cells)
	}
	for r, row := range values {
		if ctx.Err() != nil {
			return XlsxExportFile{}, ctx.Err()
		}
		for c, v := range row {
			if !validXlsxText(v) {
				return XlsxExportFile{}, fmt.Errorf("invalid stored rule text")
			}
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr(xlsxDataSheet, cell, v); err != nil {
				return XlsxExportFile{}, err
			}
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11}, Fill: excelize.Fill{Type: "pattern", Color: []string{"263B55"}, Pattern: 1}})
	if err != nil {
		return XlsxExportFile{}, err
	}
	if err = f.SetCellStyle(xlsxDataSheet, "A1", "F1", style); err != nil {
		return XlsxExportFile{}, err
	}
	if err = f.SetColWidth(xlsxDataSheet, "A", "F", 18); err != nil {
		return XlsxExportFile{}, err
	}
	if err = f.SetColWidth(xlsxDataSheet, "B", "B", 36); err != nil {
		return XlsxExportFile{}, err
	}
	if err = f.SetColWidth(xlsxDataSheet, "E", "E", 55); err != nil {
		return XlsxExportFile{}, err
	}
	if err = f.SetPanes(xlsxDataSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return XlsxExportFile{}, err
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return XlsxExportFile{}, err
	}
	if buffer.Len() > XlsxMaxBytes {
		return XlsxExportFile{}, fmt.Errorf("export file exceeds size limit")
	}
	return XlsxExportFile{FileName: "mail-recipient-rule.xlsx", ContentBase64: base64.StdEncoding.EncodeToString(buffer.Bytes())}, nil
}

func validXlsxText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool {
		return (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0xfffe || r == 0xffff
	})
}
