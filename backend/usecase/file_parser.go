package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	directParseMaxObjectSize = int64(200 * 1024 * 1024)
	directParseMaxTextBytes  = 20 * 1024 * 1024
	xlsxRowsPerTable         = 80
	xlsxMaxColumns           = 80
)

type directParseResult struct {
	Content   string
	OK        bool
	Message   string
	Truncated bool
	// SkipCrawler means the local parser made an intentional decision not to
	// parse the source, usually because the file is too large. In that case,
	// falling back to crawler would reintroduce the same large URL/file transfer
	// problem this path is designed to avoid.
	SkipCrawler bool
}

func (u *BatchImportUsecase) extractFileContentDirect(ctx context.Context, fileKey, filename string) directParseResult {
	if u.fileUsecase == nil {
		return directParseResult{Message: "文件服务未初始化"}
	}
	reportLearningStage(ctx, "file_info", "读取源文件大小和访问权限")
	info, err := u.fileUsecase.StatObject(ctx, fileKey)
	if err != nil {
		return directParseResult{Message: "读取文件信息失败：" + err.Error()}
	}
	if info.Size <= 0 {
		return directParseResult{Message: "文件为空", SkipCrawler: true}
	}
	if info.Size > directParseMaxObjectSize {
		return directParseResult{Message: fmt.Sprintf("文件超过 %d MB，已跳过正文解析", directParseMaxObjectSize/(1024*1024)), SkipCrawler: true}
	}

	reportLearningStage(ctx, "download", fmt.Sprintf("下载源文件（%.1f MB）", float64(info.Size)/(1024*1024)))
	tmp, _, err := u.fileUsecase.DownloadObjectToTemp(ctx, fileKey)
	if err != nil {
		return directParseResult{Message: "下载文件到临时目录失败：" + err.Error()}
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	reportLearningStage(ctx, "parse", "本地提取正文："+filename)
	content, err := parseLocalFileToMarkdown(ctx, tmp.Name(), filename)
	if err != nil {
		return directParseResult{Message: err.Error()}
	}
	content = normalizeParsedMarkdown(content)
	if strings.TrimSpace(content) == "" {
		return directParseResult{Message: "文件未提取到有效正文"}
	}
	content, truncated := limitParsedMarkdown(content)
	message := ""
	if truncated {
		message = fmt.Sprintf("正文已解析，但内容超过 %d MB，已截断后参与学习", directParseMaxTextBytes/(1024*1024))
	}
	return directParseResult{Content: content, OK: true, Message: message, Truncated: truncated}
}

func parseLocalFileToMarkdown(ctx context.Context, filePath, filename string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return parseTextFile(filePath, filename, true)
	case ".txt", ".log":
		return parseTextFile(filePath, filename, false)
	case ".html", ".htm":
		return parseHTMLFile(filePath, filename)
	case ".csv":
		return parseCSVFile(filePath, filename, ',')
	case ".tsv":
		return parseCSVFile(filePath, filename, '\t')
	case ".docx":
		return parseDOCXFile(filePath, filename)
	case ".pptx":
		return parsePPTXFile(filePath, filename)
	case ".xlsx":
		return parseXLSXFile(filePath, filename)
	case ".pdf":
		return parsePDFFile(filePath, filename)
	default:
		return "", fmt.Errorf("该文件类型暂不支持本地正文解析")
	}
}

func parseTextFile(filePath, filename string, markdown bool) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取文本文件失败：%w", err)
	}
	text := decodePossiblyGB18030(data)
	if markdown {
		return text, nil
	}
	return withDocumentTitle(filename, text), nil
}

func parseHTMLFile(filePath, filename string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取 HTML 文件失败：%w", err)
	}
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(
				table.WithSpanCellBehavior(table.SpanBehaviorMirror),
				table.WithNewlineBehavior(table.NewlineBehaviorPreserve),
			),
		),
	)
	md, err := conv.ConvertString(decodePossiblyGB18030(data))
	if err != nil {
		return "", fmt.Errorf("HTML 转 Markdown 失败：%w", err)
	}
	return withDocumentTitle(filename, md), nil
}

func parsePDFFile(filePath, filename string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("PDF 打开失败：%w", err)
	}
	defer f.Close()
	reader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("PDF 文字层读取失败：%w", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(reader, directParseMaxTextBytes+1)); err != nil {
		return "", fmt.Errorf("PDF 文字层读取失败：%w", err)
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("PDF 未提取到文字层内容")
	}
	return withDocumentTitle(filename, text), nil
}

func parseDOCXFile(filePath, filename string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("DOCX 打开失败：%w", err)
	}
	defer zr.Close()

	names := make([]string, 0)
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if name == "word/document.xml" ||
			(strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml")) ||
			(strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml")) ||
			name == "word/footnotes.xml" || name == "word/endnotes.xml" || name == "word/comments.xml" {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	var parts []string
	for _, name := range names {
		file := zipFileByName(zr.File, name)
		if file == nil {
			continue
		}
		text, err := extractOOXMLText(file)
		if err != nil {
			return "", fmt.Errorf("DOCX 正文解析失败：%w", err)
		}
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("DOCX 未提取到正文")
	}
	return withDocumentTitle(filename, strings.Join(parts, "\n\n")), nil
}

func parsePPTXFile(filePath, filename string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("PPTX 打开失败：%w", err)
	}
	defer zr.Close()
	names := make([]string, 0)
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			names = append(names, f.Name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return slideNumber(names[i]) < slideNumber(names[j])
	})

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(filename)
	b.WriteString("\n\n")
	for _, name := range names {
		file := zipFileByName(zr.File, name)
		if file == nil {
			continue
		}
		text, err := extractOOXMLText(file)
		if err != nil {
			return "", fmt.Errorf("PPTX 正文解析失败：%w", err)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		b.WriteString("## 第 ")
		b.WriteString(strconv.Itoa(slideNumber(name)))
		b.WriteString(" 页\n\n")
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(b.String()) == "# "+filename {
		return "", fmt.Errorf("PPTX 未提取到正文")
	}
	return b.String(), nil
}

func parseCSVFile(filePath, filename string, comma rune) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取表格文件失败：%w", err)
	}
	reader := csv.NewReader(strings.NewReader(decodePossiblyGB18030(data)))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("CSV 解析失败：%w", err)
	}
	md := rowsToMarkdown(filename, "", rows)
	if strings.TrimSpace(md) == "" {
		return "", fmt.Errorf("CSV 未提取到有效内容")
	}
	return md, nil
}

func parseXLSXFile(filePath, filename string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("XLSX 打开失败：%w", err)
	}
	defer zr.Close()

	shared, err := readXLSXSharedStrings(&zr.Reader)
	if err != nil {
		return "", err
	}
	sheets, err := readXLSXSheets(&zr.Reader)
	if err != nil {
		return "", err
	}
	if len(sheets) == 0 {
		return "", fmt.Errorf("XLSX 未发现工作表")
	}

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(filename)
	b.WriteString("\n\n")
	hasContent := false
	for _, sheet := range sheets {
		file := zipFileByName(zr.File, sheet.Path)
		if file == nil {
			continue
		}
		rows, err := readXLSXWorksheet(file, shared)
		if err != nil {
			return "", err
		}
		if len(rows) == 0 {
			continue
		}
		hasContent = true
		b.WriteString(rowsToMarkdown("", sheet.Name, rows))
		b.WriteString("\n\n")
	}
	if !hasContent {
		return "", fmt.Errorf("XLSX 未提取到有效内容")
	}
	return b.String(), nil
}

func extractOOXMLText(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	var b strings.Builder
	inText := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "t":
				inText = true
			case "tab":
				b.WriteString("\t")
			case "br", "cr":
				b.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				b.Write([]byte(t))
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n")
			}
		}
	}
	return normalizeParsedMarkdown(b.String()), nil
}

type xlsxSheet struct {
	Name string
	Path string
}

func readXLSXSharedStrings(zr *zip.Reader) ([]string, error) {
	file := zipFileByName(zr.File, "xl/sharedStrings.xml")
	if file == nil {
		return nil, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX sharedStrings 失败：%w", err)
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	var values []string
	var current strings.Builder
	inSI := false
	inT := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析 XLSX sharedStrings 失败：%w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "si":
				inSI = true
				current.Reset()
			case "t":
				if inSI {
					inT = true
				}
			}
		case xml.CharData:
			if inSI && inT {
				current.Write([]byte(t))
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "t":
				inT = false
			case "si":
				values = append(values, current.String())
				inSI = false
			}
		}
	}
	return values, nil
}

func readXLSXSheets(zr *zip.Reader) ([]xlsxSheet, error) {
	type workbookSheet struct {
		Name string
		RID  string
	}
	workbook := zipFileByName(zr.File, "xl/workbook.xml")
	if workbook == nil {
		return fallbackXLSXSheets(zr), nil
	}
	rc, err := workbook.Open()
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX workbook 失败：%w", err)
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	var workbookSheets []workbookSheet
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析 XLSX workbook 失败：%w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || strings.ToLower(start.Name.Local) != "sheet" {
			continue
		}
		var sheet workbookSheet
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "name":
				sheet.Name = attr.Value
			case "id":
				sheet.RID = attr.Value
			}
		}
		if sheet.Name == "" {
			sheet.Name = fmt.Sprintf("Sheet%d", len(workbookSheets)+1)
		}
		workbookSheets = append(workbookSheets, sheet)
	}

	rels := readXLSXWorkbookRels(zr)
	sheets := make([]xlsxSheet, 0, len(workbookSheets))
	for idx, sheet := range workbookSheets {
		target := rels[sheet.RID]
		if target == "" {
			target = fmt.Sprintf("worksheets/sheet%d.xml", idx+1)
		}
		target = strings.TrimPrefix(target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		sheets = append(sheets, xlsxSheet{Name: sheet.Name, Path: target})
	}
	return sheets, nil
}

func readXLSXWorkbookRels(zr *zip.Reader) map[string]string {
	file := zipFileByName(zr.File, "xl/_rels/workbook.xml.rels")
	if file == nil {
		return nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	rels := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rels
		}
		start, ok := token.(xml.StartElement)
		if !ok || strings.ToLower(start.Name.Local) != "relationship" {
			continue
		}
		var id, target string
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "Id":
				id = attr.Value
			case "Target":
				target = attr.Value
			}
		}
		if id != "" && target != "" {
			rels[id] = target
		}
	}
	return rels
}

func fallbackXLSXSheets(zr *zip.Reader) []xlsxSheet {
	var names []string
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml") {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	sheets := make([]xlsxSheet, 0, len(names))
	for idx, name := range names {
		sheets = append(sheets, xlsxSheet{Name: fmt.Sprintf("Sheet%d", idx+1), Path: name})
	}
	return sheets
}

func readXLSXWorksheet(file *zip.File, shared []string) ([][]string, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX 工作表失败：%w", err)
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	var rows [][]string
	var currentRow []string
	var cellType string
	var cellCol int
	var cellValue strings.Builder
	inCell := false
	inValue := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析 XLSX 工作表失败：%w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "row":
				currentRow = nil
			case "c":
				inCell = true
				cellType = ""
				cellCol = len(currentRow) + 1
				cellValue.Reset()
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "t":
						cellType = attr.Value
					case "r":
						if col := xlsxColumnIndex(attr.Value); col > 0 {
							cellCol = col
						}
					}
				}
			case "v", "t":
				if inCell {
					inValue = true
				}
			}
		case xml.CharData:
			if inCell && inValue {
				cellValue.Write([]byte(t))
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "v", "t":
				inValue = false
			case "c":
				value := decodeXLSXCellValue(cellValue.String(), cellType, shared)
				if cellCol > xlsxMaxColumns {
					inCell = false
					continue
				}
				for len(currentRow) < cellCol {
					currentRow = append(currentRow, "")
				}
				currentRow[cellCol-1] = value
				inCell = false
			case "row":
				row := trimTrailingEmptyCells(currentRow)
				if len(row) > 0 {
					rows = append(rows, row)
				}
			}
		}
	}
	return rows, nil
}

func decodeXLSXCellValue(raw, cellType string, shared []string) string {
	raw = strings.TrimSpace(raw)
	switch cellType {
	case "s":
		idx, err := strconv.Atoi(raw)
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
	case "b":
		if raw == "1" {
			return "TRUE"
		}
		if raw == "0" {
			return "FALSE"
		}
	}
	return raw
}

func rowsToMarkdown(filename, sheetName string, rows [][]string) string {
	rows = trimEmptyRows(rows)
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	if filename != "" {
		b.WriteString("# ")
		b.WriteString(filename)
		b.WriteString("\n\n")
	}
	if sheetName != "" {
		b.WriteString("## Sheet：")
		b.WriteString(sheetName)
		b.WriteString("\n\n")
	}
	header := normalizeTableRow(rows[0])
	if len(header) == 0 {
		return b.String()
	}
	dataRows := rows[1:]
	if len(dataRows) == 0 {
		dataRows = [][]string{header}
		header = make([]string, len(dataRows[0]))
		for i := range header {
			header[i] = xlsxColumnName(i + 1)
		}
	}
	for start := 0; start < len(dataRows); start += xlsxRowsPerTable {
		end := start + xlsxRowsPerTable
		if end > len(dataRows) {
			end = len(dataRows)
		}
		writeMarkdownTable(&b, header, dataRows[start:end])
		b.WriteString("\n")
	}
	return b.String()
}

func writeMarkdownTable(b *strings.Builder, header []string, rows [][]string) {
	width := len(header)
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width > xlsxMaxColumns {
		width = xlsxMaxColumns
	}
	header = fitTableRow(header, width)
	writeMarkdownTableRow(b, header)
	separator := make([]string, width)
	for i := range separator {
		separator[i] = "---"
	}
	writeMarkdownTableRow(b, separator)
	for _, row := range rows {
		writeMarkdownTableRow(b, fitTableRow(row, width))
	}
}

func writeMarkdownTableRow(b *strings.Builder, row []string) {
	b.WriteString("|")
	for _, cell := range row {
		b.WriteString(" ")
		b.WriteString(escapeMarkdownTableCell(cell))
		b.WriteString(" |")
	}
	b.WriteString("\n")
}

func escapeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

func trimEmptyRows(rows [][]string) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		row = trimTrailingEmptyCells(row)
		if len(row) > 0 {
			result = append(result, row)
		}
	}
	return result
}

func trimTrailingEmptyCells(row []string) []string {
	end := len(row)
	for end > 0 && strings.TrimSpace(row[end-1]) == "" {
		end--
	}
	return row[:end]
}

func normalizeTableRow(row []string) []string {
	row = trimTrailingEmptyCells(row)
	for i := range row {
		if strings.TrimSpace(row[i]) == "" {
			row[i] = xlsxColumnName(i + 1)
		}
	}
	return row
}

func fitTableRow(row []string, width int) []string {
	fitted := make([]string, width)
	copy(fitted, row)
	return fitted
}

func xlsxColumnIndex(cellRef string) int {
	cellRef = strings.ToUpper(strings.TrimSpace(cellRef))
	col := 0
	for _, r := range cellRef {
		if r < 'A' || r > 'Z' {
			break
		}
		col = col*26 + int(r-'A'+1)
	}
	return col
}

func xlsxColumnName(index int) string {
	if index <= 0 {
		return ""
	}
	var chars []byte
	for index > 0 {
		index--
		chars = append([]byte{byte('A' + index%26)}, chars...)
		index /= 26
	}
	return string(chars)
}

func zipFileByName(files []*zip.File, name string) *zip.File {
	for _, f := range files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

var slideNumberRegexp = regexp.MustCompile(`slide(\d+)\.xml$`)

func slideNumber(name string) int {
	matches := slideNumberRegexp.FindStringSubmatch(strings.ToLower(name))
	if len(matches) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(matches[1])
	return value
}

func decodePossiblyGB18030(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	return strings.ToValidUTF8(string(data), "")
}

func withDocumentTitle(filename, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return "# " + filename + "\n\n" + content
}

func normalizeParsedMarkdown(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	normalized := make([]string, 0, len(lines))
	emptyCount := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			emptyCount++
			if emptyCount <= 2 {
				normalized = append(normalized, "")
			}
			continue
		}
		emptyCount = 0
		normalized = append(normalized, line)
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func limitParsedMarkdown(content string) (string, bool) {
	if len(content) <= directParseMaxTextBytes {
		return content, false
	}
	limited := content[:directParseMaxTextBytes]
	for !utf8.ValidString(limited) && len(limited) > 0 {
		limited = limited[:len(limited)-1]
	}
	return strings.TrimSpace(limited) + "\n\n> 文档内容较长，系统已截断后续内容。", true
}
