package parser_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestTextParserParsesMarkdown(t *testing.T) {
	parsed, err := parser.NewTextParser().Parse(context.Background(), service.ParseInput{
		Name:        "manual.md",
		ContentType: "text/markdown",
		Body:        strings.NewReader("# Intro\n\ncontent"),
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Title != "Intro" || parsed.Content == "" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestFixedChunkerChunksAndTracksHeading(t *testing.T) {
	chunks, err := parser.NewFixedChunker().Chunk(context.Background(), service.ChunkInput{
		Content: "# Intro\n\nabcdef",
		Strategy: service.ChunkStrategy{
			"chunkSize": 6,
			"overlap":   0,
		},
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("chunks empty")
	}
	if chunks[0].Content == "" || chunks[0].TokenCount == 0 {
		t.Fatalf("chunk = %+v", chunks[0])
	}
}
