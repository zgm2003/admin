package recipientrule

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const xlsxXMLMaxElements = 100000
const xlsxMaxSharedStrings = 2 * (XlsxMaxRows + 1) * 6

type xlsxSheet struct{ name, relationshipID string }
type xlsxRelationship struct {
	kind, target  string
	ignoredMailto bool
}
type xlsxSheetBounds struct {
	lastRow int
	merged  bool
}
type xlsxArchive struct {
	parts                                   map[string]bool
	sheets                                  []xlsxSheet
	relationships                           map[string]map[string]xlsxRelationship
	worksheets                              map[string]xlsxSheetBounds
	sharedStrings, maxSharedIndex, elements int
	dataRows                                int
}

func validateXlsxArchive(ctx context.Context, content []byte) string {
	_, code := inspectXlsxArchive(ctx, content)
	return code
}

// The ZIP/XML preflight completes before Excelize unmarshals anything. It bounds
// both compressed/decompressed bytes and structural counts/coordinates, and
// validates every part, including instructions and unused relationships.
func inspectXlsxArchive(ctx context.Context, content []byte) (*xlsxArchive, string) {
	if len(content) > XlsxMaxBytes {
		return nil, "too_large"
	}
	if !bytes.HasPrefix(content, []byte("PK\x03\x04")) {
		return nil, "invalid_xlsx"
	}
	z, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(z.File) == 0 || len(z.File) > 128 {
		return nil, "invalid_xlsx"
	}
	a := &xlsxArchive{parts: map[string]bool{}, relationships: map[string]map[string]xlsxRelationship{}, worksheets: map[string]xlsxSheetBounds{}, maxSharedIndex: -1}
	total := int64(0)
	for _, entry := range z.File {
		if ctx.Err() != nil {
			return nil, "invalid_xlsx"
		}
		name := entry.Name
		if entry.UncompressedSize64 > uint64(xlsxUnzipLimit-total) {
			return nil, "too_large"
		}
		if a.parts[name] || !safeXlsxEntryName(name) || entry.Flags&1 != 0 {
			return nil, "invalid_xlsx"
		}
		a.parts[name] = true
		directory := strings.HasSuffix(name, "/")
		if directory && entry.UncompressedSize64 != 0 {
			return nil, "invalid_xlsx"
		}
		r, err := entry.Open()
		if err != nil {
			return nil, "invalid_xlsx"
		}
		raw, readErr := io.ReadAll(io.LimitReader(r, xlsxUnzipLimit-total+1))
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			return nil, "invalid_xlsx"
		}
		total += int64(len(raw))
		if total > xlsxUnzipLimit {
			return nil, "too_large"
		}
		if directory {
			if len(raw) != 0 {
				return nil, "invalid_xlsx"
			}
			continue
		}
		if !utf8.Valid(raw) {
			return nil, "invalid_xlsx"
		}
		if code := a.scanXML(ctx, name, bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})); code != "" {
			return nil, code
		}
	}
	if !a.parts["[Content_Types].xml"] || !a.parts["xl/workbook.xml"] || !a.parts["_rels/.rels"] || !a.parts["xl/_rels/workbook.xml.rels"] || a.maxSharedIndex >= a.sharedStrings {
		return nil, "invalid_xlsx"
	}
	for part, relationships := range a.relationships {
		if strings.HasPrefix(part, "xl/worksheets/_rels/") && !a.parts["xl/worksheets/"+strings.TrimSuffix(path.Base(part), ".rels")] {
			return nil, "invalid_xlsx"
		}
		for _, relationship := range relationships {
			if relationship.ignoredMailto {
				continue
			}
			if !a.parts[relationship.target] {
				return nil, "invalid_xlsx"
			}
		}
	}
	dataFound := false
	for _, sheet := range a.sheets {
		dataFound = dataFound || sheet.name == xlsxDataSheet
	}
	if !dataFound {
		return nil, "missing_sheet"
	}
	if len(a.sheets) > 2 || len(a.worksheets) != len(a.sheets) {
		return nil, "invalid_xlsx"
	}
	seenNames, seenTargets := map[string]bool{}, map[string]bool{}
	for _, sheet := range a.sheets {
		relationship, ok := a.relationships["xl/_rels/workbook.xml.rels"][sheet.relationshipID]
		bounds, exists := a.worksheets[relationship.target]
		if !ok || !exists || relationship.kind != "worksheet" || seenNames[sheet.name] || seenTargets[relationship.target] || (sheet.name != xlsxDataSheet && sheet.name != "填写说明") {
			return nil, "invalid_xlsx"
		}
		seenNames[sheet.name], seenTargets[relationship.target] = true, true
		if sheet.name == xlsxDataSheet {
			if bounds.merged {
				return nil, "invalid_xlsx"
			}
			a.dataRows = bounds.lastRow
		}
	}
	return a, ""
}

func safeXlsxPartName(name string) bool {
	if name != path.Clean(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") {
		return false
	}
	switch name {
	case "[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/sharedStrings.xml", "docProps/app.xml", "docProps/core.xml", "docProps/custom.xml":
		return true
	}
	if strings.HasPrefix(name, "xl/worksheets/_rels/sheet") && strings.HasSuffix(name, ".xml.rels") {
		return strings.Count(name, "/") == 3
	}
	return (strings.HasPrefix(name, "xl/worksheets/sheet") || strings.HasPrefix(name, "xl/theme/theme")) && strings.HasSuffix(name, ".xml") && !strings.Contains(strings.TrimPrefix(name, "xl/"), "/../") && strings.Count(name, "/") == 2
}

func safeXlsxEntryName(name string) bool {
	if strings.HasSuffix(name, "/") {
		switch name {
		case "_rels/", "docProps/", "xl/", "xl/_rels/", "xl/theme/", "xl/worksheets/", "xl/worksheets/_rels/":
			return true
		}
		return false
	}
	return safeXlsxPartName(name)
}

func (a *xlsxArchive) scanXML(ctx context.Context, name string, raw []byte) string {
	d := xml.NewDecoder(bytes.NewReader(raw))
	stack := []string{}
	rootSeen, sheetDataSeen := false, false
	worksheet := strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml")
	alternateDepth := 0
	row, column := 0, 0
	bounds := xlsxSheetBounds{}
	cellType, cellValue := "", ""
	cellHasValue, cellHasInline := false, false
	for {
		if ctx.Err() != nil {
			return "invalid_xlsx"
		}
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "invalid_xlsx"
		}
		switch element := token.(type) {
		case xml.Directive:
			return "invalid_xlsx"
		case xml.ProcInst:
			if element.Target != "xml" || rootSeen {
				return "invalid_xlsx"
			}
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(element)) != "" {
				return "invalid_xlsx"
			}
			if worksheet && len(stack) == 5 && stack[4] == "v" && stack[3] == "c" {
				if len(cellValue)+len(element) > 131068 {
					return "invalid_xlsx"
				}
				cellValue += string(element)
			}
		case xml.EndElement:
			if worksheet && element.Name.Local == "c" {
				if cellType == "s" {
					index, valid := xlsxNonnegativeInt(strings.TrimSpace(cellValue))
					if !cellHasValue || !valid || index >= xlsxMaxSharedStrings || cellHasInline {
						return "invalid_xlsx"
					}
					a.maxSharedIndex = max(a.maxSharedIndex, index)
				}
				if (cellType == "inlineStr" && cellHasValue) || (cellType != "inlineStr" && cellHasInline) {
					return "invalid_xlsx"
				}
			}
			if len(stack) == alternateDepth {
				alternateDepth = 0
			}
			stack = stack[:len(stack)-1]
		case xml.StartElement:
			a.elements++
			if a.elements > xlsxXMLMaxElements || len(stack) >= 32 || len(element.Attr) > 64 {
				return "invalid_xlsx"
			}
			local, parent := element.Name.Local, ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			} else {
				if rootSeen || !xlsxXMLRoot(name, local) {
					return "invalid_xlsx"
				}
				rootSeen = true
			}
			attrs := map[string]string{}
			for _, attr := range element.Attr {
				if _, exists := attrs[attr.Name.Local]; exists {
					return "invalid_xlsx"
				}
				attrs[attr.Name.Local] = attr.Value
				if attr.Name.Local == "ContentType" {
					v := strings.ToLower(attr.Value)
					for _, forbidden := range []string{"macro", "vba", "oleobject", "activex", "external", "embedding", "encrypt"} {
						if strings.Contains(v, forbidden) {
							return "invalid_xlsx"
						}
					}
				}
			}
			stack = append(stack, local)
			// WPS/Excel save the last local folder in a compatibility block. It is
			// inert metadata, never a path to read or a relationship to follow.
			if alternateDepth != 0 {
				choice := len(stack) == 3 && parent == "AlternateContent" && (local == "Choice" || local == "Fallback") && element.Name.Space == "http://schemas.openxmlformats.org/markup-compatibility/2006"
				folder := len(stack) == 4 && (parent == "Choice" || parent == "Fallback") && local == "absPath" && element.Name.Space == "http://schemas.microsoft.com/office/spreadsheetml/2010/11/ac"
				if !choice && !folder {
					return "invalid_xlsx"
				}
			}
			switch local {
			case "f":
				return "unsupported_formula"
			case "definedName", "externalReference", "oleObject", "control", "drawing", "legacyDrawing", "legacyDrawingHF", "ddeLink":
				return "invalid_xlsx"
			case "AlternateContent":
				if name != "xl/workbook.xml" || parent != "workbook" || len(stack) != 2 || element.Name.Space != "http://schemas.openxmlformats.org/markup-compatibility/2006" {
					return "invalid_xlsx"
				}
				alternateDepth = len(stack)
			}
			if strings.HasSuffix(name, ".rels") && local == "Relationship" {
				if parent != "Relationships" || len(stack) != 2 {
					return "invalid_xlsx"
				}
				if code := a.addRelationship(name, attrs); code != "" {
					return code
				}
			}
			if name == "xl/workbook.xml" && local == "sheet" {
				if parent != "sheets" || len(stack) != 3 || attrs["name"] == "" || attrs["id"] == "" || len(a.sheets) >= 2 {
					return "invalid_xlsx"
				}
				a.sheets = append(a.sheets, xlsxSheet{attrs["name"], attrs["id"]})
			}
			if name == "xl/sharedStrings.xml" && local == "si" {
				a.sharedStrings++
				if parent != "sst" || len(stack) != 2 || a.sharedStrings > xlsxMaxSharedStrings {
					return "invalid_xlsx"
				}
			}
			if !worksheet {
				continue
			}
			switch local {
			case "sheetData":
				if parent != "worksheet" || len(stack) != 2 || sheetDataSeen {
					return "invalid_xlsx"
				}
				sheetDataSeen = true
			case "row":
				next, valid := xlsxNonnegativeInt(attrs["r"])
				if parent != "sheetData" || len(stack) != 3 || !valid || next <= row {
					return "invalid_xlsx"
				}
				if next > XlsxMaxRows+1 {
					return "too_many_rows"
				}
				row, column, bounds.lastRow = next, 0, next
				if span := attrs["spans"]; span != "" {
					parts := strings.Split(span, ":")
					if len(parts) != 2 {
						return "invalid_columns"
					}
					start, ok1 := xlsxNonnegativeInt(parts[0])
					end, ok2 := xlsxNonnegativeInt(parts[1])
					if !ok1 || !ok2 || start < 1 || start > end || end > 6 {
						return "invalid_columns"
					}
				}
			case "c":
				col, cellRow, err := excelize.CellNameToCoordinates(attrs["r"])
				if parent != "row" || len(stack) != 4 || err != nil || cellRow != row || col <= column {
					return "invalid_xlsx"
				}
				if col > 6 {
					return "invalid_columns"
				}
				column, cellType, cellValue, cellHasValue, cellHasInline = col, attrs["t"], "", false, false
				switch cellType {
				case "", "n", "s", "str", "inlineStr", "b", "d":
				default:
					return "invalid_xlsx"
				}
			case "v", "is":
				if parent != "c" || len(stack) != 5 {
					return "invalid_xlsx"
				}
				if local == "v" {
					if cellHasValue {
						return "invalid_xlsx"
					}
					cellHasValue = true
				} else {
					if cellHasInline {
						return "invalid_xlsx"
					}
					cellHasInline = true
				}
			case "col":
				start, ok1 := xlsxNonnegativeInt(attrs["min"])
				end, ok2 := xlsxNonnegativeInt(attrs["max"])
				if !ok1 || !ok2 || start < 1 || start > end || end > 6 {
					return "invalid_columns"
				}
			case "mergeCell":
				bounds.merged = true
			}
			for _, key := range []string{"ref", "sqref", "activeCell", "topLeftCell"} {
				if value := attrs[key]; value != "" && !validXlsxRange(value) {
					return "invalid_xlsx"
				}
			}
		}
	}
	if !rootSeen || len(stack) != 0 || (worksheet && !sheetDataSeen) {
		return "invalid_xlsx"
	}
	if worksheet {
		a.worksheets[name] = bounds
	}
	return ""
}

func xlsxXMLRoot(name, root string) bool {
	switch {
	case strings.HasSuffix(name, ".rels"):
		return root == "Relationships"
	case name == "[Content_Types].xml":
		return root == "Types"
	case name == "xl/workbook.xml":
		return root == "workbook"
	case name == "xl/sharedStrings.xml":
		return root == "sst"
	case name == "xl/styles.xml":
		return root == "styleSheet"
	case strings.HasPrefix(name, "xl/worksheets/"):
		return root == "worksheet"
	case strings.HasPrefix(name, "xl/theme/"):
		return root == "theme"
	}
	return true
}

func (a *xlsxArchive) addRelationship(name string, attrs map[string]string) string {
	kind, target, id := path.Base(attrs["Type"]), attrs["Target"], attrs["Id"]
	if id == "" || target == "" || strings.Contains(target, "..") || strings.TrimSpace(target) != target {
		return "invalid_xlsx"
	}
	switch kind {
	case "officeDocument", "worksheet", "styles", "theme", "sharedStrings", "core-properties", "extended-properties", "custom-properties":
	case "hyperlink":
		if !strings.HasPrefix(name, "xl/worksheets/_rels/") || attrs["Type"] != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" || attrs["TargetMode"] != "External" || !strings.HasPrefix(strings.ToLower(target), "mailto:") || strings.ContainsAny(target, "\\\x00\r\n") {
			return "invalid_xlsx"
		}
		if a.relationships[name] == nil {
			a.relationships[name] = map[string]xlsxRelationship{}
		}
		if _, exists := a.relationships[name][id]; exists {
			return "invalid_xlsx"
		}
		// Spreadsheet editors automatically decorate email cells with mailto:
		// links. Only GetCellValue is used; this target is never read or followed.
		a.relationships[name][id] = xlsxRelationship{kind: kind, ignoredMailto: true}
		return ""
	default:
		return "invalid_xlsx"
	}
	if attrs["TargetMode"] != "" && attrs["TargetMode"] != "Internal" {
		return "invalid_xlsx"
	}
	if strings.ContainsAny(target, "\\:%?#") {
		return "invalid_xlsx"
	}
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	} else {
		base := strings.TrimSuffix(path.Dir(name), "/_rels")
		if base == "_rels" {
			base = ""
		}
		target = path.Join(base, target)
	}
	if !safeXlsxPartName(target) {
		return "invalid_xlsx"
	}
	if a.relationships[name] == nil {
		a.relationships[name] = map[string]xlsxRelationship{}
	}
	if _, exists := a.relationships[name][id]; exists {
		return "invalid_xlsx"
	}
	a.relationships[name][id] = xlsxRelationship{kind: kind, target: target}
	return ""
}

func xlsxNonnegativeInt(value string) (int, bool) {
	if value == "" || len(value) > 10 {
		return 0, false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(value)
	return n, err == nil
}

func validXlsxRange(value string) bool {
	if len(value) > 65536 {
		return false
	}
	for _, group := range strings.Fields(value) {
		cells := strings.Split(group, ":")
		if len(cells) > 2 {
			return false
		}
		for _, cell := range cells {
			c, r, err := excelize.CellNameToCoordinates(strings.ReplaceAll(cell, "$", ""))
			if err != nil || c > 6 || r > XlsxMaxRows+1 {
				return false
			}
		}
	}
	return strings.TrimSpace(value) != ""
}
