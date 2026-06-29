package parser

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type OCRRequest struct {
	DocumentName string
	ContentType  string
	Data         []byte
	RequestID    string
	UserID       string
}

type OCRResult struct {
	Text string
}

type OCRClient interface {
	ExtractText(ctx context.Context, request OCRRequest) (OCRResult, error)
}

func parseWithOCR(ctx context.Context, ocr OCRClient, request OCRRequest, unsupportedMessage string) (service.ParsedDocument, error) {
	if ocr == nil {
		return service.ParsedDocument{}, fmt.Errorf("%s", unsupportedMessage)
	}
	if len(request.Data) == 0 {
		return service.ParsedDocument{}, fmt.Errorf("document is empty")
	}
	result, err := ocr.ExtractText(ctx, request)
	if err != nil {
		return service.ParsedDocument{}, service.DependencyError("document OCR failed", err)
	}
	content := strings.TrimSpace(result.Text)
	if content == "" {
		return service.ParsedDocument{}, fmt.Errorf("document is empty")
	}
	return service.ParsedDocument{Content: content, Title: firstNonEmptyLine(content)}, nil
}

func imageContentType(name string, data []byte) string {
	ext := strings.ToLower(strings.TrimPrefix(lastExtension(name), "."))
	switch ext {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "tif", "tiff":
		return "image/tiff"
	case "webp":
		return "image/webp"
	}
	switch {
	case len(data) >= 8 && hasImageMagic(data) && data[1] == 'P':
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case strings.HasPrefix(string(data), "GIF87a") || strings.HasPrefix(string(data), "GIF89a"):
		return "image/gif"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

func lastExtension(name string) string {
	index := strings.LastIndex(name, ".")
	if index < 0 {
		return ""
	}
	return name[index:]
}
