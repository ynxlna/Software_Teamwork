package httpapi

import "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"

type runtimeConfigResponse struct {
	EmbeddingProvider    string            `json:"embeddingProvider"`
	EmbeddingModel       string            `json:"embeddingModel"`
	EmbeddingDimension   int               `json:"embeddingDimension"`
	QdrantCollection     string            `json:"qdrantCollection"`
	ParserBackend        string            `json:"parserBackend"`
	RerankProvider       string            `json:"rerankProvider"`
	RerankModel          string            `json:"rerankModel,omitempty"`
	RetrievalTopK        int               `json:"retrievalTopK"`
	ScoreThreshold       float64           `json:"scoreThreshold"`
	MaxConcurrentJobs    int               `json:"maxConcurrentJobs"`
	ProcessingTimeoutSec int               `json:"processingTimeoutSec"`
	SecretRefs           map[string]string `json:"secretRefs,omitempty"`
}

type runtimeConfigRequest struct {
	ParserBackend        *string           `json:"parserBackend,omitempty"`
	RerankProvider       *string           `json:"rerankProvider,omitempty"`
	RerankModel          *string           `json:"rerankModel,omitempty"`
	RetrievalTopK        *int              `json:"retrievalTopK,omitempty"`
	ScoreThreshold       *float64          `json:"scoreThreshold,omitempty"`
	MaxConcurrentJobs    *int              `json:"maxConcurrentJobs,omitempty"`
	ProcessingTimeoutSec *int              `json:"processingTimeoutSec,omitempty"`
	SecretRefs           map[string]string `json:"secretRefs,omitempty"`
}

type createKnowledgeBaseJobRequest struct {
	JobType string `json:"jobType"`
}

type knowledgeStatsResponse struct {
	KnowledgeBaseCount  int               `json:"knowledgeBaseCount"`
	DocumentCount       int               `json:"documentCount"`
	ChunkCount          int               `json:"chunkCount"`
	ReadyDocumentCount  int               `json:"readyDocumentCount"`
	FailedDocumentCount int               `json:"failedDocumentCount"`
	RecentUploads       []dailyUploadStat `json:"recentUploads"`
}

type dailyUploadStat struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func runtimeConfigFromDomain(cfg service.RuntimeConfig) runtimeConfigResponse {
	return runtimeConfigResponse{
		EmbeddingProvider:    cfg.EmbeddingProvider,
		EmbeddingModel:       cfg.EmbeddingModel,
		EmbeddingDimension:   cfg.EmbeddingDimension,
		QdrantCollection:     cfg.QdrantCollection,
		ParserBackend:        cfg.ParserBackend,
		RerankProvider:       cfg.RerankProvider,
		RerankModel:          cfg.RerankModel,
		RetrievalTopK:        cfg.RetrievalTopK,
		ScoreThreshold:       cfg.ScoreThreshold,
		MaxConcurrentJobs:    cfg.MaxConcurrentJobs,
		ProcessingTimeoutSec: cfg.ProcessingTimeoutSec,
		SecretRefs:           cloneStringMap(cfg.SecretRefs),
	}
}

func knowledgeStatsFromDomain(stats service.KnowledgeStats) knowledgeStatsResponse {
	uploads := make([]dailyUploadStat, 0, len(stats.RecentUploads))
	for _, item := range stats.RecentUploads {
		uploads = append(uploads, dailyUploadStat{Date: item.Date, Count: item.Count})
	}
	return knowledgeStatsResponse{
		KnowledgeBaseCount:  stats.KnowledgeBaseCount,
		DocumentCount:       stats.DocumentCount,
		ChunkCount:          stats.ChunkCount,
		ReadyDocumentCount:  stats.ReadyDocumentCount,
		FailedDocumentCount: stats.FailedDocumentCount,
		RecentUploads:       uploads,
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
