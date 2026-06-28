package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

const maxParsedTextBytes = 8 << 20

type TextParser struct{}

func NewTextParser() *TextParser {
	return &TextParser{}
}

func (p *TextParser) Parse(ctx context.Context, input service.ParseInput) (service.ParsedDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.ParsedDocument{}, err
	}
	if !isTextLike(input.Name, input.ContentType) {
		return service.ParsedDocument{}, fmt.Errorf("unsupported content type")
	}
	limited := io.LimitReader(input.Body, maxParsedTextBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return service.ParsedDocument{}, err
	}
	if len(data) > maxParsedTextBytes {
		return service.ParsedDocument{}, fmt.Errorf("document is too large for text parser")
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return service.ParsedDocument{}, fmt.Errorf("document is empty")
	}
	if !utf8.ValidString(content) {
		return service.ParsedDocument{}, fmt.Errorf("document is not valid utf-8")
	}
	return service.ParsedDocument{
		Content: content,
		Title:   firstMarkdownHeading(content),
	}, nil
}

func isTextLike(name string, contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(contentType))
	switch mediaType {
	case "", "text/plain", "text/markdown", "application/markdown", "application/x-markdown":
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".txt" || ext == ".md" || ext == ".markdown"
}

func firstMarkdownHeading(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}
