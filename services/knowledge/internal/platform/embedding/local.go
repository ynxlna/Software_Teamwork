package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type LocalHasher struct {
	provider  string
	model     string
	dimension int
}

func NewLocalHasher(provider string, model string, dimension int) *LocalHasher {
	if provider == "" {
		provider = "local_hashing"
	}
	if model == "" {
		model = "local_hashing"
	}
	if dimension <= 0 {
		dimension = 384
	}
	return &LocalHasher{provider: provider, model: model, dimension: dimension}
}

func (e *LocalHasher) Embed(ctx context.Context, request service.EmbeddingRequest) (service.EmbeddingResult, error) {
	if err := ctx.Err(); err != nil {
		return service.EmbeddingResult{}, err
	}
	vectors := make([][]float32, 0, len(request.Texts))
	for _, text := range request.Texts {
		if text == "" {
			return service.EmbeddingResult{}, fmt.Errorf("text must not be empty")
		}
		vectors = append(vectors, hashVector(text, e.dimension))
	}
	return service.EmbeddingResult{
		Vectors:   vectors,
		Provider:  e.provider,
		Model:     e.model,
		Dimension: e.dimension,
	}, nil
}

func hashVector(text string, dimension int) []float32 {
	vector := make([]float32, dimension)
	seed := []byte(text)
	for i := 0; i < dimension; i++ {
		sum := sha256.Sum256(append(seed, byte(i), byte(i>>8)))
		value := binary.BigEndian.Uint32(sum[:4])
		vector[i] = (float32(value%2000) / 1000.0) - 1.0
	}
	return vector
}
