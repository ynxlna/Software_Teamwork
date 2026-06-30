package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

type SimpleDOCXGenerator struct{}

func NewSimpleDOCXGenerator() *SimpleDOCXGenerator {
	return &SimpleDOCXGenerator{}
}

func (g *SimpleDOCXGenerator) GenerateDOCX(ctx context.Context, report Report, sections []ReportSection) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":          contentTypesXML,
		"_rels/.rels":                  packageRelsXML,
		"word/_rels/document.xml.rels": documentRelsXML,
		"word/document.xml":            buildDocumentXML(report, sections),
		"word/styles.xml":              stylesXML,
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer, err := zipWriter.Create(name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, fmt.Errorf("create docx part %s: %w", name, err)
		}
		if _, err := writer.Write([]byte(files[name])); err != nil {
			_ = zipWriter.Close()
			return nil, fmt.Errorf("write docx part %s: %w", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close docx package: %w", err)
	}
	return buffer.Bytes(), nil
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const packageRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const documentRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:qFormat/>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="32"/></w:rPr>
  </w:style>
</w:styles>`

func buildDocumentXML(report Report, sections []ReportSection) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	writeParagraph(&b, report.Name, true)
	writeParagraph(&b, report.Topic, false)
	ordered := append([]ReportSection(nil), sections...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].SortOrder == ordered[j].SortOrder {
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		}
		return ordered[i].SortOrder < ordered[j].SortOrder
	})
	for _, section := range ordered {
		title := strings.TrimSpace(section.Title)
		if title != "" {
			if section.Numbering != "" {
				title = section.Numbering + " " + title
			}
			writeParagraph(&b, title, true)
		}
		for _, para := range splitParagraphs(section.Content) {
			writeParagraph(&b, para, false)
		}
		for _, table := range section.Tables {
			writeParagraph(&b, flattenTable(table), false)
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func writeParagraph(b *strings.Builder, text string, bold bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b.WriteString(`<w:p>`)
	if bold {
		b.WriteString(`<w:pPr><w:pStyle w:val="Heading1"/></w:pPr>`)
	}
	b.WriteString(`<w:r>`)
	b.WriteString(`<w:t>`)
	xml.EscapeText(b, []byte(text))
	b.WriteString(`</w:t></w:r></w:p>`)
}

func splitParagraphs(content string) []string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func flattenTable(table map[string]any) string {
	if len(table) == 0 {
		return ""
	}
	parts := make([]string, 0, len(table))
	for key, value := range table {
		parts = append(parts, fmt.Sprintf("%s: %v", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
