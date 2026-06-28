package source

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type MemorySourceReader struct {
	mu      sync.RWMutex
	sources map[string]memorySource
}

type memorySource struct {
	content     string
	contentType string
	sizeBytes   int64
}

func NewMemorySourceReader() *MemorySourceReader {
	return &MemorySourceReader{sources: map[string]memorySource{}}
}

func (r *MemorySourceReader) Put(fileID string, content string, contentType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[fileID] = memorySource{
		content:     content,
		contentType: contentType,
		sizeBytes:   int64(len(content)),
	}
}

func (r *MemorySourceReader) ReadSource(ctx context.Context, fileID string) (service.SourceDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.SourceDocument{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	source, exists := r.sources[fileID]
	if !exists {
		return service.SourceDocument{}, fmt.Errorf("source not found")
	}
	return service.SourceDocument{
		Body:        io.NopCloser(strings.NewReader(source.content)),
		ContentType: source.contentType,
		SizeBytes:   source.sizeBytes,
	}, nil
}
