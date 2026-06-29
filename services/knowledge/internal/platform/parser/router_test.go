package parser_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestRouterParsesTextMarkdownAndOfficeDocuments(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		contentType string
		body        []byte
		wantTitle   string
		wantParts   []string
	}{
		{
			name:        "markdown",
			fileName:    "manual.md",
			contentType: "text/markdown",
			body:        []byte("# Intro\n\ncontent"),
			wantTitle:   "Intro",
			wantParts:   []string{"# Intro", "content"},
		},
		{
			name:        "docx",
			fileName:    "manual.docx",
			contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			body: officeZip(t, map[string]string{
				"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Safety Manual</w:t></w:r></w:p><w:p><w:r><w:t>Breaker checklist</w:t></w:r></w:p></w:body></w:document>`,
			}),
			wantTitle: "Safety Manual",
			wantParts: []string{"Safety Manual", "Breaker checklist"},
		},
		{
			name:        "docx content beats misleading type",
			fileName:    "manual.pdf",
			contentType: "application/pdf",
			body: officeZip(t, map[string]string{
				"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Actual DOCX</w:t></w:r></w:p></w:body></w:document>`,
			}),
			wantTitle: "Actual DOCX",
			wantParts: []string{"Actual DOCX"},
		},
		{
			name:        "pptx relationship order",
			fileName:    "training.pptx",
			contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			body: officeZip(t, map[string]string{
				"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId1"/></p:sldIdLst></p:presentation>`,
				"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="slides/slide1.xml"/><Relationship Id="rId2" Target="slides/slide2.xml"/></Relationships>`,
				"ppt/slides/slide2.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Second slide</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
				"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Intro slide</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
			}),
			wantTitle: "Second slide",
			wantParts: []string{"Slide 1", "Second slide", "Slide 2", "Intro slide"},
		},
		{
			name:        "xlsx workbook order",
			fileName:    "assets.xlsx",
			contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			body: officeZip(t, map[string]string{
				"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets><sheet name="Second" sheetId="2" r:id="rId2" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/><sheet name="First" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/></sheets></workbook>`,
				"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet2.xml"/></Relationships>`,
				"xl/sharedStrings.xml":       `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Asset</t></si><si><t>Status</t></si></sst>`,
				"xl/worksheets/sheet1.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row></sheetData></worksheet>`,
				"xl/worksheets/sheet2.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>Transformer</v></c><c r="B1"><is><t>Ready</t></is></c></row></sheetData></worksheet>`,
			}),
			wantTitle: "Sheet 1",
			wantParts: []string{"Sheet 1", "Transformer", "Ready", "Sheet 2", "Asset", "Status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parser.NewRouter().Parse(context.Background(), service.ParseInput{
				Name:        tt.fileName,
				ContentType: tt.contentType,
				Body:        bytes.NewReader(tt.body),
				SizeBytes:   int64(len(tt.body)),
			})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if parsed.Title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", parsed.Title, tt.wantTitle)
			}
			searchFrom := 0
			for _, part := range tt.wantParts {
				offset := strings.Index(parsed.Content[searchFrom:], part)
				if offset < 0 {
					t.Fatalf("content = %q, want ordered part %q after offset %d", parsed.Content, part, searchFrom)
				}
				searchFrom += offset + len(part)
			}
		})
	}
}

func TestRouterRejectsUnsupportedAndInvalidDocumentsSafely(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		contentType string
		body        []byte
	}{
		{
			name:        "pdf",
			fileName:    "scan.pdf",
			contentType: "application/pdf",
			body:        []byte("%PDF-1.7\nsecret document text"),
		},
		{
			name:        "png",
			fileName:    "photo.png",
			contentType: "image/png",
			body:        []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 's', 'e', 'c', 'r', 'e', 't'},
		},
		{
			name:        "unknown binary",
			fileName:    "blob.bin",
			contentType: "application/octet-stream",
			body:        []byte{0x00, 0x01, 0x02, 's', 'e', 'c', 'r', 'e', 't'},
		},
		{
			name:        "unknown utf8 without content type",
			fileName:    "blob.bin",
			contentType: "",
			body:        []byte("secret but not declared text"),
		},
		{
			name:        "legacy office",
			fileName:    "legacy.doc",
			contentType: "application/msword",
			body:        []byte{0xd0, 0xcf, 0x11, 0xe0, 's', 'e', 'c', 'r', 'e', 't'},
		},
		{
			name:        "damaged docx",
			fileName:    "broken.docx",
			contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			body:        []byte("not a zip but contains secret"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.NewRouter().Parse(context.Background(), service.ParseInput{
				Name:        tt.fileName,
				ContentType: tt.contentType,
				Body:        bytes.NewReader(tt.body),
				SizeBytes:   int64(len(tt.body)),
			})
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked source content: %v", err)
			}
		})
	}
}

func TestRouterRejectsEmptyAndOversizedDocuments(t *testing.T) {
	_, err := parser.NewRouter().Parse(context.Background(), service.ParseInput{
		Name:        "empty.txt",
		ContentType: "text/plain",
		Body:        strings.NewReader("   \n\t"),
		SizeBytes:   5,
	})
	if err == nil {
		t.Fatal("Parse() empty text error = nil, want error")
	}

	ocr := &fakeOCRClient{textByName: map[string]string{"empty.pdf": "must not parse empty file"}}
	_, err = parser.NewRouterWithOCR(ocr).Parse(context.Background(), service.ParseInput{
		Name:        "empty.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader(nil),
		SizeBytes:   0,
	})
	if err == nil {
		t.Fatal("Parse() empty pdf error = nil, want error")
	}
	if len(ocr.requests) != 0 {
		t.Fatalf("ocr requests = %+v, want none for empty document", ocr.requests)
	}

	oversized := bytes.Repeat([]byte("a"), 8<<20+1)
	_, err = parser.NewRouter().Parse(context.Background(), service.ParseInput{
		Name:        "large.txt",
		ContentType: "text/plain",
		Body:        bytes.NewReader(oversized),
		SizeBytes:   int64(len(oversized)),
	})
	if err == nil {
		t.Fatal("Parse() oversized text error = nil, want error")
	}
}

func TestRouterParsesPDFThroughOCR(t *testing.T) {
	ocr := &fakeOCRClient{
		textByName: map[string]string{
			"scan.pdf": "Transformer OCR label",
		},
	}
	parsed, err := parser.NewRouterWithOCR(ocr).Parse(context.Background(), service.ParseInput{
		Name:        "scan.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader([]byte("%PDF-1.7\nbinary page image")),
		SizeBytes:   26,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Title != "Transformer OCR label" || !strings.Contains(parsed.Content, "Transformer OCR label") {
		t.Fatalf("parsed = %+v", parsed)
	}
	if len(ocr.requests) != 1 || ocr.requests[0].ContentType != "application/pdf" {
		t.Fatalf("ocr requests = %+v", ocr.requests)
	}
}

func TestRouterParsesPPTXImageTextThroughOCR(t *testing.T) {
	image := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'l', 'a', 'b', 'e', 'l'}
	ocr := &fakeOCRClient{
		textByName: map[string]string{
			"ppt/media/image1.png": "Pump Station 3",
		},
	}
	pptx := officeZip(t, map[string]string{
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:pic><p:blipFill><a:blip r:embed="rId2"/></p:blipFill></p:pic></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Target="../media/image1.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/></Relationships>`,
		"ppt/media/image1.png":             string(image),
	})

	parsed, err := parser.NewRouterWithOCR(ocr).Parse(context.Background(), service.ParseInput{
		Name:        "training.pptx",
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Body:        bytes.NewReader(pptx),
		SizeBytes:   int64(len(pptx)),
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Title != "Pump Station 3" || !strings.Contains(parsed.Content, "Pump Station 3") {
		t.Fatalf("parsed = %+v", parsed)
	}
	if len(ocr.requests) != 1 || ocr.requests[0].DocumentName != "ppt/media/image1.png" || ocr.requests[0].ContentType != "image/png" {
		t.Fatalf("ocr requests = %+v", ocr.requests)
	}
}

func TestRouterSkipsUnusedAndExternalPPTXImageRelationships(t *testing.T) {
	image := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'l', 'a', 'b', 'e', 'l'}
	ocr := &fakeOCRClient{
		textByName: map[string]string{
			"ppt/media/rendered.png": "Visible Safety Label",
		},
	}
	pptx := officeZip(t, map[string]string{
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:pic><p:blipFill><a:blip r:embed="rVisible"/></p:blipFill></p:pic></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rUnused" Target="../media/unused.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/>
<Relationship Id="rExternal" Target="https://files.internal/secret.png" TargetMode="External" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/>
<Relationship Id="rTraversal" Target="../../media/outside.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/>
<Relationship Id="rVisible" Target="../media/rendered.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/>
</Relationships>`,
		"ppt/media/unused.png":   string(image),
		"ppt/media/rendered.png": string(image),
	})

	parsed, err := parser.NewRouterWithOCR(ocr).Parse(context.Background(), service.ParseInput{
		Name:        "training.pptx",
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Body:        bytes.NewReader(pptx),
		SizeBytes:   int64(len(pptx)),
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !strings.Contains(parsed.Content, "Visible Safety Label") {
		t.Fatalf("parsed = %+v", parsed)
	}
	if len(ocr.requests) != 1 || ocr.requests[0].DocumentName != "ppt/media/rendered.png" {
		t.Fatalf("ocr requests = %+v", ocr.requests)
	}
}

func officeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}
	return buf.Bytes()
}

type fakeOCRClient struct {
	textByName map[string]string
	requests   []parser.OCRRequest
}

func (c *fakeOCRClient) ExtractText(ctx context.Context, request parser.OCRRequest) (parser.OCRResult, error) {
	c.requests = append(c.requests, request)
	text, ok := c.textByName[request.DocumentName]
	if !ok {
		return parser.OCRResult{}, fmt.Errorf("unexpected OCR request")
	}
	return parser.OCRResult{Text: text}, nil
}
