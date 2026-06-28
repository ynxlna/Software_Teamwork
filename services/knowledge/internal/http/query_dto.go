package httpapi

import "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"

type knowledgeQueryRequest struct {
	Query            string            `json:"query"`
	KnowledgeBaseIDs []string          `json:"knowledgeBaseIds,omitempty"`
	TopK             int               `json:"topK,omitempty"`
	ScoreThreshold   *float64          `json:"scoreThreshold,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	MetadataFilter   map[string]string `json:"metadataFilter,omitempty"`
	Rerank           bool              `json:"rerank,omitempty"`
	RerankTopN       *int              `json:"rerankTopN,omitempty"`
}

type knowledgeQuerySummary struct {
	ID      string                 `json:"id"`
	Query   string                 `json:"query"`
	Results []knowledgeQueryResult `json:"results"`
	Trace   knowledgeQueryTrace    `json:"trace"`
}

type knowledgeQueryResult struct {
	Score           float64  `json:"score"`
	PointID         string   `json:"pointId,omitempty"`
	KnowledgeBaseID string   `json:"knowledgeBaseId"`
	DocumentID      string   `json:"documentId"`
	ChunkID         string   `json:"chunkId"`
	DocumentName    string   `json:"documentName"`
	SectionPath     *string  `json:"sectionPath,omitempty"`
	ChunkIndex      *int     `json:"chunkIndex,omitempty"`
	ChunkType       *string  `json:"chunkType,omitempty"`
	ContentPreview  string   `json:"contentPreview"`
	Tags            []string `json:"tags,omitempty"`
}

type knowledgeQueryTrace struct {
	EmbeddingProvider  string  `json:"embeddingProvider"`
	EmbeddingModel     string  `json:"embeddingModel"`
	EmbeddingDimension int     `json:"embeddingDimension"`
	QdrantCollection   string  `json:"qdrantCollection"`
	SearchTopK         int     `json:"searchTopK"`
	ScoreThreshold     float64 `json:"scoreThreshold"`
	HitCount           int     `json:"hitCount"`
	Rerank             bool    `json:"rerank"`
	RerankTopN         *int    `json:"rerankTopN,omitempty"`
}

func knowledgeQueryFromDomain(query service.KnowledgeQuerySummary) knowledgeQuerySummary {
	results := make([]knowledgeQueryResult, 0, len(query.Results))
	for _, result := range query.Results {
		results = append(results, knowledgeQueryResult{
			Score:           result.Score,
			PointID:         result.PointID,
			KnowledgeBaseID: result.KnowledgeBaseID,
			DocumentID:      result.DocumentID,
			ChunkID:         result.ChunkID,
			DocumentName:    result.DocumentName,
			SectionPath:     cloneStringPtr(result.SectionPath),
			ChunkIndex:      cloneIntPtr(result.ChunkIndex),
			ChunkType:       cloneStringPtr(result.ChunkType),
			ContentPreview:  result.ContentPreview,
			Tags:            append([]string(nil), result.Tags...),
		})
	}
	return knowledgeQuerySummary{
		ID:      query.ID,
		Query:   query.Query,
		Results: results,
		Trace: knowledgeQueryTrace{
			EmbeddingProvider:  query.Trace.EmbeddingProvider,
			EmbeddingModel:     query.Trace.EmbeddingModel,
			EmbeddingDimension: query.Trace.EmbeddingDimension,
			QdrantCollection:   query.Trace.QdrantCollection,
			SearchTopK:         query.Trace.SearchTopK,
			ScoreThreshold:     query.Trace.ScoreThreshold,
			HitCount:           query.Trace.HitCount,
			Rerank:             query.Trace.Rerank,
			RerankTopN:         cloneIntPtr(query.Trace.RerankTopN),
		},
	}
}
