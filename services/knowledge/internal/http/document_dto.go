package httpapi

import (
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type documentSummary struct {
	ID              string                 `json:"id"`
	KnowledgeBaseID string                 `json:"knowledgeBaseId"`
	Name            string                 `json:"name"`
	ContentType     *string                `json:"contentType,omitempty"`
	SizeBytes       int64                  `json:"sizeBytes,omitempty"`
	Status          service.DocumentStatus `json:"status"`
	ErrorCode       *string                `json:"errorCode,omitempty"`
	ErrorMessage    *string                `json:"errorMessage,omitempty"`
	ChunkCount      int                    `json:"chunkCount"`
	Tags            []string               `json:"tags,omitempty"`
	ParserBackend   *string                `json:"parserBackend,omitempty"`
	CreatedBy       string                 `json:"createdBy,omitempty"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       *string                `json:"updatedAt,omitempty"`
	JobID           *string                `json:"jobId,omitempty"`
}

type documentChunk struct {
	ID                 string         `json:"id"`
	KnowledgeBaseID    string         `json:"knowledgeBaseId"`
	DocumentID         string         `json:"documentId"`
	ChunkIndex         int            `json:"chunkIndex"`
	SectionPath        *string        `json:"sectionPath,omitempty"`
	Content            string         `json:"content"`
	TokenCount         int            `json:"tokenCount"`
	ChunkType          *string        `json:"chunkType,omitempty"`
	QdrantPointID      *string        `json:"qdrantPointId,omitempty"`
	EmbeddingProvider  *string        `json:"embeddingProvider,omitempty"`
	EmbeddingDimension *int           `json:"embeddingDimension,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CreatedAt          string         `json:"createdAt"`
}

func documentSummaryFromDomain(doc service.KnowledgeDocument) documentSummary {
	var updatedAt *string
	if doc.UpdatedAt != nil {
		formatted := doc.UpdatedAt.UTC().Format(time.RFC3339)
		updatedAt = &formatted
	}
	return documentSummary{
		ID:              doc.ID,
		KnowledgeBaseID: doc.KnowledgeBaseID,
		Name:            doc.Name,
		ContentType:     cloneStringPtr(doc.ContentType),
		SizeBytes:       doc.SizeBytes,
		Status:          doc.Status,
		ErrorCode:       cloneStringPtr(doc.ErrorCode),
		ErrorMessage:    cloneStringPtr(doc.ErrorMessage),
		ChunkCount:      doc.ChunkCount,
		Tags:            append([]string(nil), doc.Tags...),
		ParserBackend:   cloneStringPtr(doc.ParserBackend),
		CreatedBy:       doc.CreatedBy,
		CreatedAt:       doc.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       updatedAt,
		JobID:           cloneStringPtr(doc.CurrentJobID),
	}
}

func documentListFromDomain(list service.DocumentList) []documentSummary {
	items := make([]documentSummary, 0, len(list.Items))
	for _, doc := range list.Items {
		items = append(items, documentSummaryFromDomain(doc))
	}
	return items
}

func documentChunkFromDomain(chunk service.DocumentChunk) documentChunk {
	return documentChunk{
		ID:                 chunk.ID,
		KnowledgeBaseID:    chunk.KnowledgeBaseID,
		DocumentID:         chunk.DocumentID,
		ChunkIndex:         chunk.ChunkIndex,
		SectionPath:        cloneStringPtr(chunk.SectionPath),
		Content:            chunk.Content,
		TokenCount:         chunk.TokenCount,
		ChunkType:          cloneStringPtr(chunk.ChunkType),
		QdrantPointID:      cloneStringPtr(chunk.QdrantPointID),
		EmbeddingProvider:  cloneStringPtr(chunk.EmbeddingProvider),
		EmbeddingDimension: cloneIntPtr(chunk.EmbeddingDimension),
		Metadata:           cloneJSONMap(chunk.Metadata),
		CreatedAt:          chunk.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func documentChunkListFromDomain(list service.ChunkList) []documentChunk {
	items := make([]documentChunk, 0, len(list.Items))
	for _, chunk := range list.Items {
		items = append(items, documentChunkFromDomain(chunk))
	}
	return items
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
