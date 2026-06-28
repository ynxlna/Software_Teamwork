package httpapi

import (
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type knowledgeBaseRequest struct {
	ID                string         `json:"id,omitempty"`
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	DocType           string         `json:"docType,omitempty"`
	ChunkStrategy     map[string]any `json:"chunkStrategy,omitempty"`
	RetrievalStrategy map[string]any `json:"retrievalStrategy,omitempty"`
}

type updateKnowledgeBaseRequest struct {
	Name              *string        `json:"name,omitempty"`
	Description       *string        `json:"description,omitempty"`
	DocType           *string        `json:"docType,omitempty"`
	ChunkStrategy     map[string]any `json:"chunkStrategy,omitempty"`
	RetrievalStrategy map[string]any `json:"retrievalStrategy,omitempty"`
}

type knowledgeBaseSummary struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	DocType           string         `json:"docType"`
	ChunkStrategy     map[string]any `json:"chunkStrategy"`
	RetrievalStrategy map[string]any `json:"retrievalStrategy"`
	DocumentCount     int            `json:"documentCount"`
	ChunkCount        int            `json:"chunkCount"`
	CreatedBy         string         `json:"createdBy,omitempty"`
	CreatedAt         string         `json:"createdAt"`
	UpdatedAt         string         `json:"updatedAt"`
}

func knowledgeBaseSummaryFromDomain(base service.KnowledgeBase) knowledgeBaseSummary {
	return knowledgeBaseSummary{
		ID:                base.ID,
		Name:              base.Name,
		Description:       base.Description,
		DocType:           base.DocType,
		ChunkStrategy:     cloneJSONMap(base.ChunkStrategy),
		RetrievalStrategy: cloneJSONMap(base.RetrievalStrategy),
		DocumentCount:     base.DocumentCount,
		ChunkCount:        base.ChunkCount,
		CreatedBy:         base.CreatedBy,
		CreatedAt:         base.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         base.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func knowledgeBaseListFromDomain(list service.KnowledgeBaseList) []knowledgeBaseSummary {
	items := make([]knowledgeBaseSummary, 0, len(list.Items))
	for _, base := range list.Items {
		items = append(items, knowledgeBaseSummaryFromDomain(base))
	}
	return items
}

func pageInfoFromDomain(page service.Page) map[string]int {
	return map[string]int{
		"page":     page.Page,
		"pageSize": page.PageSize,
		"total":    page.Total,
	}
}

func cloneJSONMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
