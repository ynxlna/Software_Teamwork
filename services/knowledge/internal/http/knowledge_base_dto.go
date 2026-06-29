package httpapi

import (
	"encoding/json"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type createKnowledgeBaseRequest struct {
	ID                *string          `json:"id"`
	Name              string           `json:"name"`
	Description       *string          `json:"description"`
	DocType           *string          `json:"docType"`
	ChunkStrategy     *json.RawMessage `json:"chunkStrategy"`
	RetrievalStrategy *json.RawMessage `json:"retrievalStrategy"`
}

type updateKnowledgeBaseRequest struct {
	Name              *string          `json:"name"`
	Description       *string          `json:"description"`
	DocType           *string          `json:"docType"`
	ChunkStrategy     *json.RawMessage `json:"chunkStrategy"`
	RetrievalStrategy *json.RawMessage `json:"retrievalStrategy"`
}

type knowledgeBaseSummary struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	DocType           string          `json:"docType"`
	ChunkStrategy     json.RawMessage `json:"chunkStrategy"`
	RetrievalStrategy json.RawMessage `json:"retrievalStrategy"`
	DocumentCount     int64           `json:"documentCount"`
	ChunkCount        int64           `json:"chunkCount"`
	CreatedBy         string          `json:"createdBy,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

func knowledgeBaseFromDomain(kb service.KnowledgeBase) knowledgeBaseSummary {
	return knowledgeBaseSummary{
		ID:                kb.ID,
		Name:              kb.Name,
		Description:       kb.Description,
		DocType:           kb.DocType,
		ChunkStrategy:     cloneRaw(kb.ChunkStrategy),
		RetrievalStrategy: cloneRaw(kb.RetrievalStrategy),
		DocumentCount:     kb.DocumentCount,
		ChunkCount:        kb.ChunkCount,
		CreatedBy:         kb.CreatedBy,
		CreatedAt:         kb.CreatedAt,
		UpdatedAt:         kb.UpdatedAt,
	}
}

func knowledgeBasesFromDomain(items []service.KnowledgeBase) []knowledgeBaseSummary {
	out := make([]knowledgeBaseSummary, 0, len(items))
	for _, item := range items {
		out = append(out, knowledgeBaseFromDomain(item))
	}
	return out
}
