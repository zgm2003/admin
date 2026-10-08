package recipientrule

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"slices"
	"strings"
	"testing"
)

func rewriteXlsxPart(t *testing.T, input XlsxImportInput, name string, edit func(string) string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(input.ContentBase64)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	found := false
	for _, entry := range z.File {
		if entry.Name != name {
			if err := w.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		found = true
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		p, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(p, edit(string(data))); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatalf("part %s not found", name)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestXlsxImportInputRejectsUnsafeBasenames(t *testing.T) {
	for _, name := range []string{".xlsx", "C:rules.xlsx", "bad\tname.xlsx", "bad\x7fname.xlsx", "bad?name.xlsx", "bad*name.xlsx", "bad|name.xlsx", "CON.xlsx", "LPT1.xlsx", "../rules.xlsx", `C:\rules.xlsx`, "rules.xls", "rules.xlsm", "rules.csv"} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeXlsxImportInput(XlsxImportInput{FileName: name, ContentBase64: "UEsDBA=="}); err == nil {
				t.Fatalf("unsafe filename accepted: %q", name)
			}
		})
	}
}

func TestXlsxArchiveRejectsMalformedStructureBeforeLibraryAllocation(t *testing.T) {
	input := xlsxInput(t, importHeader+"email,a@example.com,deny,name,,1\n")
	for _, tc := range []struct{ name, part, from, to, code string }{
		{"unknown sheet", "xl/workbook.xml", `name="导入数据"`, `name="其他数据"`, "missing_sheet"},
		{"extra sheet", "xl/workbook.xml", `</sheets>`, `<sheet name="隐藏数据" sheetId="2" r:id="rId1"/></sheets>`, "invalid_xlsx"},
		{"too many sheets", "xl/workbook.xml", `</sheets>`, `<sheet name="填写说明" sheetId="2" r:id="rId1"/><sheet name="第三页" sheetId="3" r:id="rId1"/></sheets>`, "invalid_xlsx"},
		{"duplicate sheet", "xl/workbook.xml", `</sheets>`, `<sheet name="导入数据" sheetId="2" r:id="rId1"/></sheets>`, "invalid_xlsx"},
		{"huge column metadata", "xl/worksheets/sheet1.xml", `<sheetData>`, `<cols><col min="1" max="2147483647"/></cols><sheetData>`, "invalid_columns"},
		{"huge dimension", "xl/worksheets/sheet1.xml", `ref="A1"`, `ref="A1:XFD1048576"`, "invalid_xlsx"},
		{"duplicate sheetData", "xl/worksheets/sheet1.xml", `</sheetData>`, `</sheetData><sheetData/>`, "invalid_xlsx"},
		{"row outside data", "xl/worksheets/sheet1.xml", `</sheetData>`, `</sheetData><row r="3"><c r="A3" t="inlineStr"><is><t>bad</t></is></c></row>`, "invalid_xlsx"},
		{"unclosed tail", "xl/worksheets/sheet1.xml", `</worksheet>`, ``, "invalid_xlsx"},
		{"second XML root", "xl/worksheets/sheet1.xml", `</worksheet>`, `</worksheet><worksheet/>`, "invalid_xlsx"},
		{"content after XML", "xl/worksheets/sheet1.xml", `</worksheet>`, `</worksheet>garbage`, "invalid_xlsx"},
		{"duplicate attr", "xl/worksheets/sheet1.xml", `r="A1"`, `r="A1" r="A1"`, "invalid_xlsx"},
		{"macro relationship", "xl/_rels/workbook.xml.rels", `/worksheet"`, `/vbaProject"`, "invalid_xlsx"},
		{"internal path traversal", "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="../../xl/worksheets/sheet1.xml"`, "invalid_xlsx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := rewriteXlsxPart(t, input, tc.part, func(raw string) string {
				if !strings.Contains(raw, tc.from) {
					t.Fatalf("fixture does not contain %q", tc.from)
				}
				return strings.Replace(raw, tc.from, tc.to, 1)
			})
			// Check the preflight directly: unsafe coordinates must never reach the library.
			if code := validateXlsxArchive(context.Background(), data); code != tc.code {
				t.Fatalf("archive error=%q want=%q", code, tc.code)
			}
		})
	}
}

func TestXlsxMalformedCellCannotBecomePartialPreviewSuccess(t *testing.T) {
	input := xlsxInput(t, importHeader+"email,a@example.com,deny,valid,,1\n")
	for _, value := range []string{"999999999", "-1", "not-an-index"} {
		data := rewriteXlsxPart(t, input, "xl/worksheets/sheet1.xml", func(raw string) string {
			return strings.Replace(raw, `</sheetData>`, `<row r="3"><c r="A3" t="s"><v>`+value+`</v></c></row></sheetData>`, 1)
		})
		p := parseXlsx(context.Background(), data)
		if len(p.Rows) != 0 || !slices.Equal(p.Errors, []string{"invalid_xlsx"}) {
			t.Fatalf("malformed shared-string index %q yielded a partial preview: %+v", value, p)
		}
	}
}

func wpsXlsxFixture(t *testing.T) []byte {
	t.Helper()
	input := xlsxInput(t, importHeader+"email,user@example.com,deny,valid,,1\n")
	data := rewriteXlsxPart(t, input, "xl/workbook.xml", func(raw string) string {
		return strings.Replace(raw, "</workbook>", `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice Requires="x15"><x15ac:absPath url="C:\\Users\\Downloads\\" xmlns:x15ac="http://schemas.microsoft.com/office/spreadsheetml/2010/11/ac"/></mc:Choice></mc:AlternateContent></workbook>`, 1)
	})
	data = rewriteXlsxPart(t, XlsxImportInput{ContentBase64: base64.StdEncoding.EncodeToString(data)}, "xl/worksheets/sheet1.xml", func(raw string) string {
		return strings.Replace(raw, "</worksheet>", `<hyperlinks><hyperlink ref="B2" r:id="rId1"/></hyperlinks></worksheet>`, 1)
	})
	var out bytes.Buffer
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(&out)
	for _, entry := range z.File {
		if err := w.Copy(entry); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"xl/", "xl/worksheets/", "xl/worksheets/_rels/"} {
		if _, err := w.Create(name); err != nil {
			t.Fatal(err)
		}
	}
	p, err := w.Create("xl/worksheets/_rels/sheet1.xml.rels")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(p, `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="mailto:user@example.com" TargetMode="External"/></Relationships>`); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestXlsxImportAcceptsWPSCompatibilityPartsAndMailtoHyperlink(t *testing.T) {
	preview := parseXlsx(context.Background(), wpsXlsxFixture(t))
	if len(preview.Errors) != 0 || len(preview.Rows) != 1 || preview.Rows[0].Data == nil {
		t.Fatalf("WPS-compatible workbook rejected: %+v", preview)
	}
	if preview.Rows[0].Data.Pattern != "user@example.com" || preview.Rows[0].Data.Action != ActionDeny {
		t.Fatalf("cell values changed by presentation metadata: %+v", preview.Rows[0].Data)
	}
}

func TestXlsxWPSCompatibilityStillRejectsActiveOrUnsafeParts(t *testing.T) {
	input := XlsxImportInput{ContentBase64: base64.StdEncoding.EncodeToString(wpsXlsxFixture(t))}
	for _, tc := range []struct{ name, part, from, to string }{
		{"HTTP link", "xl/worksheets/_rels/sheet1.xml.rels", "mailto:user@example.com", "https://example.com/book.xlsx"},
		{"file link", "xl/worksheets/_rels/sheet1.xml.rels", "mailto:user@example.com", "file:///C:/private/file.xlsx"},
		{"external data", "xl/worksheets/_rels/sheet1.xml.rels", "/hyperlink", "/externalLink"},
		{"wrong relationship mode", "xl/worksheets/_rels/sheet1.xml.rels", `TargetMode="External"`, `TargetMode="Internal"`},
		{"compatibility drawing", "xl/workbook.xml", "<x15ac:absPath", "<drawing"},
		{"compatibility namespace", "xl/workbook.xml", `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"`, `<mc:AlternateContent xmlns:mc="https://example.com/untrusted"`},
		{"cached formula", "xl/worksheets/sheet1.xml", `</sheetData>`, `<row r="3"><c r="A3"><f>1+1</f><v>2</v></c></row></sheetData>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := rewriteXlsxPart(t, input, tc.part, func(raw string) string {
				if !strings.Contains(raw, tc.from) {
					t.Fatalf("missing fixture fragment %q", tc.from)
				}
				return strings.Replace(raw, tc.from, tc.to, 1)
			})
			if code := validateXlsxArchive(context.Background(), content); code == "" {
				t.Fatal("unsafe workbook accepted")
			}
		})
	}
	for _, directory := range []string{"../xl/", "/xl/", "xl/../", `xl\worksheets/`, "xl/embeddings/"} {
		if safeXlsxEntryName(directory) {
			t.Errorf("unsafe ZIP directory accepted: %q", directory)
		}
	}
}
