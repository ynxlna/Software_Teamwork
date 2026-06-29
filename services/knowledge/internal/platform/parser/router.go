package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type documentFormat string

const (
	formatText          documentFormat = "text"
	formatDOCX          documentFormat = "docx"
	formatPPTX          documentFormat = "pptx"
	formatXLSX          documentFormat = "xlsx"
	formatPDF           documentFormat = "pdf"
	formatImage         documentFormat = "image"
	formatLegacyOffice  documentFormat = "legacy_office"
	formatUnknownBinary documentFormat = "unknown_binary"
)

type Router struct {
	text *TextParser
	ocr  OCRClient
}

func NewRouter() *Router {
	return &Router{text: NewTextParser()}
}

func NewRouterWithOCR(ocr OCRClient) *Router {
	return &Router{text: NewTextParser(), ocr: ocr}
}

func (r *Router) Parse(ctx context.Context, input service.ParseInput) (service.ParsedDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.ParsedDocument{}, err
	}
	data, err := readBoundedDocument(input.Body)
	if err != nil {
		return service.ParsedDocument{}, err
	}
	format, archive, err := detectFormat(input.Name, input.ContentType, data)
	if err != nil {
		return service.ParsedDocument{}, err
	}
	switch format {
	case formatText:
		return r.text.Parse(ctx, service.ParseInput{
			Name:        input.Name,
			ContentType: input.ContentType,
			Body:        bytes.NewReader(data),
			SizeBytes:   int64(len(data)),
		})
	case formatDOCX:
		return parseDOCX(archive)
	case formatPPTX:
		return parsePPTX(ctx, archive, r.ocr, ocrRequestContext{
			requestID: input.RequestID,
			userID:    input.UserID,
		})
	case formatXLSX:
		return parseXLSX(archive)
	case formatPDF:
		return parseWithOCR(ctx, r.ocr, OCRRequest{
			DocumentName: input.Name,
			ContentType:  "application/pdf",
			Data:         data,
			RequestID:    input.RequestID,
			UserID:       input.UserID,
		}, "pdf parsing is not supported without external parser")
	case formatImage:
		return parseWithOCR(ctx, r.ocr, OCRRequest{
			DocumentName: input.Name,
			ContentType:  imageContentType(input.Name, data),
			Data:         data,
			RequestID:    input.RequestID,
			UserID:       input.UserID,
		}, "image OCR is not supported without external parser")
	case formatLegacyOffice:
		return service.ParsedDocument{}, fmt.Errorf("legacy Office document parsing is not supported")
	default:
		return service.ParsedDocument{}, fmt.Errorf("unsupported document format")
	}
}

func readBoundedDocument(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, fmt.Errorf("document body is required")
	}
	limited := io.LimitReader(body, maxParsedTextBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxParsedTextBytes {
		return nil, fmt.Errorf("document is too large for parser")
	}
	return data, nil
}

func detectFormat(name string, contentType string, data []byte) (documentFormat, *zip.Reader, error) {
	mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(contentType))
	ext := strings.ToLower(filepath.Ext(name))

	if looksLikeZip(data) {
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", nil, fmt.Errorf("document archive could not be read")
		}
		if hasZipFile(archive, "word/document.xml") {
			return formatDOCX, archive, nil
		}
		if hasZipFile(archive, "ppt/presentation.xml") || hasZipPrefix(archive, "ppt/slides/") {
			return formatPPTX, archive, nil
		}
		if hasZipFile(archive, "xl/workbook.xml") || hasZipPrefix(archive, "xl/worksheets/") {
			return formatXLSX, archive, nil
		}
		return formatUnknownBinary, nil, nil
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return formatPDF, nil, nil
	}
	if hasImageMagic(data) {
		return formatImage, nil, nil
	}
	if hasLegacyOfficeMagic(data) {
		return formatLegacyOffice, nil, nil
	}
	if isOOXMLHint(mediaType, ext) {
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", nil, fmt.Errorf("document archive could not be read")
		}
		switch {
		case mediaType == docxMediaType || ext == ".docx":
			return formatDOCX, archive, nil
		case mediaType == pptxMediaType || ext == ".pptx":
			return formatPPTX, archive, nil
		case mediaType == xlsxMediaType || ext == ".xlsx":
			return formatXLSX, archive, nil
		}
	}
	if isPDFHint(mediaType, ext) {
		return formatPDF, nil, nil
	}
	if isImageHint(mediaType, ext) {
		return formatImage, nil, nil
	}
	if isLegacyOfficeHint(mediaType, ext) {
		return formatLegacyOffice, nil, nil
	}
	if isTextFormatHint(mediaType, ext) {
		return formatText, nil, nil
	}
	return formatUnknownBinary, nil, nil
}

const (
	docxMediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	pptxMediaType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	xlsxMediaType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

func isOOXMLHint(mediaType string, ext string) bool {
	switch mediaType {
	case docxMediaType, pptxMediaType, xlsxMediaType:
		return true
	}
	return ext == ".docx" || ext == ".pptx" || ext == ".xlsx"
}

func isTextFormatHint(mediaType string, ext string) bool {
	switch mediaType {
	case "text/plain", "text/markdown", "application/markdown", "application/x-markdown":
		return true
	}
	return ext == ".txt" || ext == ".md" || ext == ".markdown"
}

func isPDFHint(mediaType string, ext string) bool {
	return mediaType == "application/pdf" || ext == ".pdf"
}

func isImageHint(mediaType string, ext string) bool {
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return true
	}
	return false
}

func hasImageMagic(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) ||
		bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) ||
		bytes.HasPrefix(data, []byte("GIF87a")) ||
		bytes.HasPrefix(data, []byte("GIF89a")) ||
		bytes.HasPrefix(data, []byte("BM")) ||
		bytes.HasPrefix(data, []byte("II*\x00")) ||
		bytes.HasPrefix(data, []byte("MM\x00*")) ||
		(len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP")
}

func isLegacyOfficeHint(mediaType string, ext string) bool {
	switch mediaType {
	case "application/msword", "application/vnd.ms-excel", "application/vnd.ms-powerpoint":
		return true
	}
	if ext == ".doc" || ext == ".xls" || ext == ".ppt" {
		return true
	}
	return false
}

func hasLegacyOfficeMagic(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
}

func looksLikeZip(data []byte) bool {
	return bytes.HasPrefix(data, []byte("PK\x03\x04")) ||
		bytes.HasPrefix(data, []byte("PK\x05\x06")) ||
		bytes.HasPrefix(data, []byte("PK\x07\x08"))
}

func hasZipFile(archive *zip.Reader, name string) bool {
	for _, file := range archive.File {
		if file.Name == name {
			return true
		}
	}
	return false
}

func hasZipPrefix(archive *zip.Reader, prefix string) bool {
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, prefix) {
			return true
		}
	}
	return false
}
