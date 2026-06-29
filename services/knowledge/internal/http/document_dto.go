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
	SizeBytes       *int64                 `json:"sizeBytes,omitempty"`
	Status          service.DocumentStatus `json:"status"`
	ErrorCode       *string                `json:"errorCode,omitempty"`
	ErrorMessage    *string                `json:"errorMessage,omitempty"`
	ChunkCount      int64                  `json:"chunkCount"`
	Tags            []string               `json:"tags,omitempty"`
	ParserBackend   *string                `json:"parserBackend,omitempty"`
	CreatedBy       string                 `json:"createdBy,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	JobID           *string                `json:"jobId,omitempty"`
}

func documentFromDomain(doc service.KnowledgeDocument) documentSummary {
	tags := append([]string(nil), doc.Tags...)
	return documentSummary{
		ID:              doc.ID,
		KnowledgeBaseID: doc.KnowledgeBaseID,
		Name:            doc.Name,
		ContentType:     doc.ContentType,
		SizeBytes:       doc.SizeBytes,
		Status:          doc.Status,
		ErrorCode:       doc.ErrorCode,
		ErrorMessage:    doc.ErrorMessage,
		ChunkCount:      doc.ChunkCount,
		Tags:            tags,
		ParserBackend:   doc.ParserBackend,
		CreatedBy:       doc.CreatedBy,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
		JobID:           doc.CurrentJobID,
	}
}

func documentsFromDomain(items []service.KnowledgeDocument) []documentSummary {
	out := make([]documentSummary, 0, len(items))
	for _, item := range items {
		out = append(out, documentFromDomain(item))
	}
	return out
}
