package recipientrule

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"github.com/xuri/excelize/v2"
	"os"
	"slices"
	"strings"
	"testing"
)

// Compact fixtures use a text matrix notation only in tests. Runtime accepts XLSX only.
func xlsxInput(t *testing.T, source string) XlsxImportInput {
	t.Helper()
	r := csv.NewReader(strings.NewReader(source))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		for col, mapping := range map[int]map[string]string{0: {"email": "邮箱", "domain": "域名"}, 2: {"allow": "允许", "deny": "拒绝"}, 5: {"0": "停用", "1": "启用"}} {
			if col < len(row) {
				if v, ok := mapping[row[col]]; ok {
					row[col] = v
				}
			}
		}
	}
	return matrixXlsx(t, rows)
}
func matrixXlsx(t *testing.T, rows [][]string) XlsxImportInput {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", xlsxDataSheet); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			if err := f.SetCellStr(xlsxDataSheet, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return XlsxImportInput{FileName: "rules.xlsx", ContentBase64: base64.StdEncoding.EncodeToString(b.Bytes())}
}
func xlsxRows(t *testing.T, encoded string) [][]string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows(xlsxDataSheet, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func TestXlsxRejectsUnsafeFilesAndSparseCoordinates(t *testing.T) {
	cases := []struct{ name, cell, formula, part, raw, want string }{
		{name: "formula", cell: "B2", formula: "1+1", want: "unsupported_formula"},
		{name: "sparse rows", cell: "A1048576", want: "too_many_rows"},
		{name: "sparse columns", cell: "XFD2", want: "invalid_columns"},
		{name: "macro", part: "xl/vbaProject.bin", raw: "vba", want: "invalid_xlsx"},
		{name: "external link", part: "xl/_rels/extra.rels", raw: `<Relationships><Relationship TargetMode="External" Target="https://example.com" /></Relationships>`, want: "invalid_xlsx"},
		{name: "zip bomb", part: "xl/extra.xml", raw: strings.Repeat("x", xlsxUnzipLimit+1), want: "too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			_ = f.SetSheetName("Sheet1", xlsxDataSheet)
			for j, v := range xlsxHeader {
				cell, _ := excelize.CoordinatesToCellName(j+1, 1)
				_ = f.SetCellStr(xlsxDataSheet, cell, v)
			}
			if tc.cell != "" {
				if tc.formula != "" {
					_ = f.SetCellFormula(xlsxDataSheet, tc.cell, tc.formula)
				} else {
					_ = f.SetCellStr(xlsxDataSheet, tc.cell, "value")
				}
			}
			b, err := f.WriteToBuffer()
			if err != nil {
				t.Fatal(err)
			}
			content := b.Bytes()
			if tc.part != "" {
				reader, _ := zip.NewReader(bytes.NewReader(content), int64(len(content)))
				var out bytes.Buffer
				w := zip.NewWriter(&out)
				for _, e := range reader.File {
					if err := w.Copy(e); err != nil {
						t.Fatal(err)
					}
				}
				p, _ := w.Create(tc.part)
				_, _ = p.Write([]byte(tc.raw))
				_ = w.Close()
				content = out.Bytes()
			}
			p := parseXlsx(context.Background(), content)
			if !slices.Contains(p.Errors, tc.want) {
				t.Fatalf("preview=%+v want=%s", p, tc.want)
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte("类型,邮箱/域名"), []byte("not zip")} {
		p := parseXlsx(context.Background(), raw)
		if !slices.Contains(p.Errors, "invalid_xlsx") {
			t.Fatal(p)
		}
	}
}
func TestXlsxBlankTemplateDoesNotImportExamples(t *testing.T) {
	data, err := os.ReadFile("../../../../../../docs/templates/mail-recipient-rule-import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	p := parseXlsx(context.Background(), data)
	if len(p.Rows) != 0 || !slices.Equal(p.Errors, []string{"empty"}) {
		t.Fatalf("template=%+v", p)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if !slices.Equal(f.GetSheetList(), []string{"填写说明", "导入数据"}) {
		t.Fatal(f.GetSheetList())
	}
	rules, err := f.GetDataValidations(xlsxDataSheet)
	if err != nil || len(rules) != 3 {
		t.Fatalf("dropdowns=%v err=%v", rules, err)
	}
	for _, rule := range rules {
		if !strings.HasSuffix(rule.Sqref, "1001") {
			t.Fatal(rule.Sqref)
		}
	}
	for c, v := range []string{"邮箱", "USER@example.com", "拒绝", "测试", "", "停用"} {
		cell, _ := excelize.CoordinatesToCellName(c+1, 4)
		_ = f.SetCellStr(xlsxDataSheet, cell, v)
	}
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	p = parseXlsx(context.Background(), b.Bytes())
	if len(p.Errors) != 0 || len(p.Rows) != 1 || p.Rows[0].Line != 4 || p.Rows[0].Data == nil || p.Rows[0].Data.Pattern != "user@example.com" || p.Rows[0].RawValues[1] != "USER@example.com" || len(p.Rows[0].Errors) != 0 {
		t.Fatalf("filled template=%+v", p)
	}
}
func TestXlsxRowsValidateChineseLabelsAndLimits(t *testing.T) {
	for _, tc := range []struct {
		col         int
		value, code string
	}{{0, "email", "invalid_scope"}, {0, "prefix", "invalid_scope"}, {1, "bad", "invalid_pattern"}, {2, "0", "invalid_action"}, {3, "", "invalid_name"}, {3, strings.Repeat("名", 129), "invalid_name"}, {4, strings.Repeat("注", 513), "invalid_remark"}, {5, "1", "invalid_status"}} {
		t.Run(fmt.Sprintf("%d-%s", tc.col, tc.code), func(t *testing.T) {
			row := []string{"邮箱", "a@example.com", "拒绝", "name", "", "启用"}
			row[tc.col] = tc.value
			in := matrixXlsx(t, [][]string{xlsxHeader, row})
			data, _ := decodeXlsxImportInput(in)
			p := parseXlsx(context.Background(), data)
			if len(p.Rows) != 1 || !slices.Contains(p.Rows[0].Errors, tc.code) {
				t.Fatal(p)
			}
		})
	}
}
