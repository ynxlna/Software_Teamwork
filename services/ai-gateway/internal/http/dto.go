package httpapi

import (
	"encoding/json"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
)

type modelProfileResponse struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Purpose           service.Purpose  `json:"purpose"`
	Provider          service.Provider `json:"provider"`
	BaseURL           string           `json:"baseUrl"`
	Model             string           `json:"model"`
	Enabled           bool             `json:"enabled"`
	IsDefault         bool             `json:"isDefault"`
	TimeoutMS         int              `json:"timeoutMs"`
	APIKeyConfigured  bool             `json:"apiKeyConfigured"`
	SupportsStreaming bool             `json:"supportsStreaming"`
	Dimensions        *int             `json:"dimensions"`
	TopN              *int             `json:"topN"`
	DefaultParameters json.RawMessage  `json:"defaultParameters"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
}

type createModelProfileRequest struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Purpose           service.Purpose  `json:"purpose"`
	Provider          service.Provider `json:"provider"`
	BaseURL           string           `json:"baseUrl"`
	Model             string           `json:"model"`
	APIKey            string           `json:"apiKey"`
	Enabled           *bool            `json:"enabled"`
	IsDefault         *bool            `json:"isDefault"`
	TimeoutMS         *int             `json:"timeoutMs"`
	SupportsStreaming *bool            `json:"supportsStreaming"`
	Dimensions        *int             `json:"dimensions"`
	TopN              *int             `json:"topN"`
	DefaultParameters json.RawMessage  `json:"defaultParameters"`
}

type updateModelProfileRequest struct {
	Name              *string           `json:"name"`
	Provider          *service.Provider `json:"provider"`
	BaseURL           *string           `json:"baseUrl"`
	Model             *string           `json:"model"`
	APIKey            *string           `json:"apiKey"`
	Enabled           *bool             `json:"enabled"`
	IsDefault         *bool             `json:"isDefault"`
	TimeoutMS         *int              `json:"timeoutMs"`
	SupportsStreaming *bool             `json:"supportsStreaming"`
	Dimensions        *int              `json:"dimensions"`
	TopN              *int              `json:"topN"`
	DefaultParameters *json.RawMessage  `json:"defaultParameters"`
}

type embeddingRequest struct {
	ProfileID      string   `json:"profile_id"`
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     *int     `json:"dimensions"`
	EncodingFormat string   `json:"encoding_format"`
	User           string   `json:"user"`
}

type rerankingRequest struct {
	ProfileID string              `json:"profile_id"`
	Model     string              `json:"model"`
	Query     string              `json:"query"`
	Documents []rerankingDocument `json:"documents"`
	TopN      *int                `json:"top_n"`
	Metadata  map[string]string   `json:"metadata"`
}

type rerankingDocument struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func profilesFromDomain(items []service.ModelProfile) []modelProfileResponse {
	out := make([]modelProfileResponse, len(items))
	for i, item := range items {
		out[i] = profileFromDomain(item)
	}
	return out
}

func profileFromDomain(profile service.ModelProfile) modelProfileResponse {
	defaultParameters := profile.DefaultParameters
	if len(defaultParameters) == 0 {
		defaultParameters = json.RawMessage(`{}`)
	}
	return modelProfileResponse{
		ID:                profile.ID,
		Name:              profile.Name,
		Purpose:           profile.Purpose,
		Provider:          profile.Provider,
		BaseURL:           profile.BaseURL,
		Model:             profile.Model,
		Enabled:           profile.Enabled,
		IsDefault:         profile.IsDefault,
		TimeoutMS:         profile.TimeoutMS,
		APIKeyConfigured:  profile.APIKeyConfigured,
		SupportsStreaming: profile.SupportsStreaming,
		Dimensions:        cloneIntPtr(profile.Dimensions),
		TopN:              cloneIntPtr(profile.TopN),
		DefaultParameters: append(json.RawMessage(nil), defaultParameters...),
		CreatedAt:         profile.CreatedAt,
		UpdatedAt:         profile.UpdatedAt,
	}
}

func createInputFromRequest(payload createModelProfileRequest) service.CreateModelProfileInput {
	return service.CreateModelProfileInput{
		ID:                payload.ID,
		Name:              payload.Name,
		Purpose:           payload.Purpose,
		Provider:          payload.Provider,
		BaseURL:           payload.BaseURL,
		Model:             payload.Model,
		APIKey:            payload.APIKey,
		Enabled:           payload.Enabled,
		IsDefault:         payload.IsDefault,
		TimeoutMS:         payload.TimeoutMS,
		SupportsStreaming: payload.SupportsStreaming,
		Dimensions:        payload.Dimensions,
		TopN:              payload.TopN,
		DefaultParameters: payload.DefaultParameters,
	}
}

func updateInputFromRequest(id string, payload updateModelProfileRequest) service.UpdateModelProfileInput {
	return service.UpdateModelProfileInput{
		ID:                id,
		Name:              payload.Name,
		Provider:          payload.Provider,
		BaseURL:           payload.BaseURL,
		Model:             payload.Model,
		APIKey:            payload.APIKey,
		Enabled:           payload.Enabled,
		IsDefault:         payload.IsDefault,
		TimeoutMS:         payload.TimeoutMS,
		SupportsStreaming: payload.SupportsStreaming,
		Dimensions:        payload.Dimensions,
		TopN:              payload.TopN,
		DefaultParameters: payload.DefaultParameters,
	}
}

func embeddingInputFromRequest(payload embeddingRequest) service.EmbeddingInput {
	return service.EmbeddingInput{
		Model:          payload.Model,
		ProfileID:      payload.ProfileID,
		Input:          append([]string(nil), payload.Input...),
		Dimensions:     cloneIntPtr(payload.Dimensions),
		EncodingFormat: payload.EncodingFormat,
		User:           payload.User,
	}
}

func rerankingInputFromRequest(payload rerankingRequest) service.RerankingInput {
	documents := make([]service.RerankingDocument, len(payload.Documents))
	for i, document := range payload.Documents {
		documents[i] = service.RerankingDocument{
			ID:   document.ID,
			Text: document.Text,
		}
	}
	return service.RerankingInput{
		Model:     payload.Model,
		ProfileID: payload.ProfileID,
		Query:     payload.Query,
		Documents: documents,
		TopN:      cloneIntPtr(payload.TopN),
		Metadata:  cloneStringMap(payload.Metadata),
	}
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]string, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
