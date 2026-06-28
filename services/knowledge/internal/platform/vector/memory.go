package vector

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type MemoryIndex struct {
	mu     sync.RWMutex
	points map[string]service.VectorPoint
}

func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{points: map[string]service.VectorPoint{}}
}

func (i *MemoryIndex) Upsert(ctx context.Context, points []service.VectorPoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, point := range points {
		i.points[point.ID] = clonePoint(point)
	}
	return nil
}

func (i *MemoryIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for id, point := range i.points {
		if point.Payload["document_id"] == documentID {
			delete(i.points, id)
		}
	}
	return nil
}

func (i *MemoryIndex) Search(ctx context.Context, request service.VectorSearchRequest) ([]service.VectorSearchHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()

	limit := request.Limit
	if limit < 1 {
		limit = 10
	}
	hits := make([]service.VectorSearchHit, 0, len(i.points))
	for _, point := range i.points {
		if !matchesKnowledgeBase(point.Payload, request.KnowledgeBaseIDs) {
			continue
		}
		if !matchesTags(point.Payload, request.Tags) {
			continue
		}
		if !matchesMetadata(point.Payload, request.MetadataFilter) {
			continue
		}
		score := cosineSimilarity(request.Vector, point.Vector)
		if score < request.ScoreThreshold {
			continue
		}
		hits = append(hits, service.VectorSearchHit{
			ID:      point.ID,
			Score:   score,
			Payload: clonePayload(point.Payload),
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (i *MemoryIndex) Point(id string) (service.VectorPoint, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	point, ok := i.points[id]
	return clonePoint(point), ok
}

func clonePoint(point service.VectorPoint) service.VectorPoint {
	point.Vector = append([]float32(nil), point.Vector...)
	if point.Payload != nil {
		point.Payload = clonePayload(point.Payload)
	}
	return point
}

func clonePayload(payload map[string]any) map[string]any {
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		switch typed := value.(type) {
		case []string:
			clone[key] = append([]string(nil), typed...)
		case map[string]any:
			copied := make(map[string]any, len(typed))
			for nestedKey, nestedValue := range typed {
				copied[nestedKey] = nestedValue
			}
			clone[key] = copied
		default:
			clone[key] = value
		}
	}
	return clone
}

func matchesKnowledgeBase(payload map[string]any, ids []string) bool {
	if len(ids) == 0 {
		return true
	}
	value, _ := payload["knowledge_base_id"].(string)
	for _, id := range ids {
		if value == id {
			return true
		}
	}
	return false
}

func matchesTags(payload map[string]any, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	payloadTags := stringsFromPayload(payload["tags"])
	if len(payloadTags) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(payloadTags))
	for _, tag := range payloadTags {
		set[tag] = struct{}{}
	}
	for _, tag := range tags {
		if _, ok := set[tag]; !ok {
			return false
		}
	}
	return true
}

func matchesMetadata(payload map[string]any, filter map[string]string) bool {
	if len(filter) == 0 {
		return true
	}
	metadata, _ := payload["metadata"].(map[string]any)
	for key, expected := range filter {
		if metadata == nil || asString(metadata[key]) != expected {
			return false
		}
	}
	return true
}

func stringsFromPayload(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func cosineSimilarity(a []float32, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
