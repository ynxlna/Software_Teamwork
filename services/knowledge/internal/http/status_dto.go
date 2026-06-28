package httpapi

import "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type readyResponse struct {
	Service            string `json:"service"`
	Status             string `json:"status"`
	Version            string `json:"version"`
	Environment        string `json:"environment"`
	StorageBackend     string `json:"storageBackend"`
	EmbeddingProvider  string `json:"embeddingProvider"`
	EmbeddingModel     string `json:"embeddingModel"`
	EmbeddingDimension int    `json:"embeddingDimension"`
	QdrantCollection   string `json:"qdrantCollection"`
}

func healthResponseFromDomain(status service.HealthStatus) healthResponse {
	return healthResponse{
		Service: status.Service,
		Status:  status.Status,
	}
}

func readyResponseFromDomain(status service.ReadyStatus) readyResponse {
	return readyResponse{
		Service:            status.Service,
		Status:             status.Status,
		Version:            status.Version,
		Environment:        status.Environment,
		StorageBackend:     status.StorageBackend,
		EmbeddingProvider:  status.EmbeddingProvider,
		EmbeddingModel:     status.EmbeddingModel,
		EmbeddingDimension: status.EmbeddingDimension,
		QdrantCollection:   status.QdrantCollection,
	}
}
