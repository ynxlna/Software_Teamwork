package parser

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type FixedChunker struct{}

func NewFixedChunker() *FixedChunker {
	return &FixedChunker{}
}

func (c *FixedChunker) Chunk(ctx context.Context, input service.ChunkInput) ([]service.ChunkSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, fmt.Errorf("content is empty")
	}
	chunkSize := intFromStrategy(input.Strategy, "chunkSize", service.DefaultChunkStrategySize)
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be positive")
	}
	overlap := intFromStrategy(input.Strategy, "overlap", service.DefaultChunkStrategyOverlap)
	if overlap < 0 {
		return nil, fmt.Errorf("overlap must be non-negative")
	}
	if overlap >= chunkSize {
		overlap = 0
	}

	runes := []rune(content)
	chunks := []service.ChunkSpec{}
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[start:end]))
		if text != "" {
			chunkType := "text"
			sectionPath := currentHeading(contentPrefix(runes, start))
			spec := service.ChunkSpec{
				SectionPath: sectionPath,
				Content:     text,
				TokenCount:  estimateTokens(text),
				ChunkType:   &chunkType,
				Metadata: map[string]any{
					"charStart": start,
					"charEnd":   end,
				},
			}
			chunks = append(chunks, spec)
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = end
		}
	}
	return chunks, serviceValidateChunks(chunks)
}

func intFromStrategy(strategy service.ChunkStrategy, key string, fallback int) int {
	value, ok := strategy[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float64:
		if typed == float64(int(typed)) {
			return int(typed)
		}
	case float32:
		if typed == float32(int(typed)) {
			return int(typed)
		}
	}
	return fallback
}

func estimateTokens(text string) int {
	words := strings.Fields(text)
	if len(words) > 0 {
		return len(words)
	}
	count := utf8.RuneCountInString(text)
	if count == 0 {
		return 0
	}
	return (count + 1) / 2
}

func contentPrefix(runes []rune, end int) string {
	if end <= 0 {
		return ""
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[:end])
}

func currentHeading(prefix string) *string {
	lines := strings.Split(prefix, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if heading != "" {
				return &heading
			}
		}
	}
	return nil
}

func serviceValidateChunks(chunks []service.ChunkSpec) error {
	if len(chunks) == 0 {
		return fmt.Errorf("must produce at least one chunk")
	}
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			return fmt.Errorf("chunk content must not be empty")
		}
	}
	return nil
}
