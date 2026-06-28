package vector

import (
	"context"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestMemoryIndexSearchFiltersAndSorts(t *testing.T) {
	index := NewMemoryIndex()
	if err := index.Upsert(context.Background(), []service.VectorPoint{
		{
			ID:     "point_1",
			Vector: []float32{1, 0},
			Payload: map[string]any{
				"knowledge_base_id": "kb_1",
				"document_id":       "doc_1",
				"chunk_id":          "chunk_1",
				"tags":              []string{"policy"},
				"metadata":          map[string]any{"plant": "A"},
			},
		},
		{
			ID:     "point_2",
			Vector: []float32{0, 1},
			Payload: map[string]any{
				"knowledge_base_id": "kb_2",
				"document_id":       "doc_2",
				"chunk_id":          "chunk_2",
				"tags":              []string{"policy"},
				"metadata":          map[string]any{"plant": "A"},
			},
		},
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	hits, err := index.Search(context.Background(), service.VectorSearchRequest{
		Vector:           []float32{1, 0},
		KnowledgeBaseIDs: []string{"kb_1"},
		Tags:             []string{"policy"},
		MetadataFilter:   map[string]string{"plant": "A"},
		Limit:            10,
		ScoreThreshold:   0.5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "point_1" {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Score < 0.99 {
		t.Fatalf("score = %f", hits[0].Score)
	}
}

func TestMemoryIndexDeleteByDocument(t *testing.T) {
	index := NewMemoryIndex()
	if err := index.Upsert(context.Background(), []service.VectorPoint{
		{ID: "point_1", Vector: []float32{1}, Payload: map[string]any{"document_id": "doc_1"}},
		{ID: "point_2", Vector: []float32{1}, Payload: map[string]any{"document_id": "doc_2"}},
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := index.DeleteByDocument(context.Background(), "doc_1"); err != nil {
		t.Fatalf("DeleteByDocument() error = %v", err)
	}
	if _, ok := index.Point("point_1"); ok {
		t.Fatal("point_1 still exists")
	}
	if _, ok := index.Point("point_2"); !ok {
		t.Fatal("point_2 missing")
	}
}
