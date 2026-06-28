package vector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestQdrantClientEnsureCollectionCreatesMissingCollection(t *testing.T) {
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/knowledge_chunks":
			http.NotFound(w, r)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/knowledge_chunks":
			createCalled = true
			var body struct {
				Vectors struct {
					Size     int    `json:"size"`
					Distance string `json:"distance"`
				} `json:"vectors"`
			}
			decodeRequest(t, r, &body)
			if body.Vectors.Size != 384 || body.Vectors.Distance != "Cosine" {
				t.Fatalf("body = %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewQdrantClient(QdrantConfig{BaseURL: server.URL, Collection: "knowledge_chunks", Dimension: 384})
	if err != nil {
		t.Fatalf("NewQdrantClient() error = %v", err)
	}
	if err := client.EnsureCollection(context.Background()); err != nil {
		t.Fatalf("EnsureCollection() error = %v", err)
	}
	if !createCalled {
		t.Fatal("create collection was not called")
	}
}

func TestQdrantClientUpsertDeleteAndSearch(t *testing.T) {
	var upsertCalled bool
	var deleteCalled bool
	var searchCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("api-key header = %q", got)
		}
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/knowledge_chunks/points":
			upsertCalled = true
			if r.URL.Query().Get("wait") != "true" {
				t.Fatalf("wait = %q", r.URL.RawQuery)
			}
			var body struct {
				Points []struct {
					ID      string         `json:"id"`
					Vector  []float32      `json:"vector"`
					Payload map[string]any `json:"payload"`
				} `json:"points"`
			}
			decodeRequest(t, r, &body)
			if len(body.Points) != 1 || body.Points[0].Payload["document_id"] != "doc_1" {
				t.Fatalf("upsert body = %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/knowledge_chunks/points/delete":
			deleteCalled = true
			var body map[string]any
			decodeRequest(t, r, &body)
			if body["filter"] == nil {
				t.Fatalf("delete body = %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/knowledge_chunks/points/search":
			searchCalled = true
			var body map[string]any
			decodeRequest(t, r, &body)
			if body["with_payload"] != true || body["filter"] == nil {
				t.Fatalf("search body = %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":[{"id":"point_1","score":0.91,"payload":{"chunk_id":"chunk_1"}}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewQdrantClient(QdrantConfig{BaseURL: server.URL, APIKey: "test-key", Collection: "knowledge_chunks", Dimension: 2})
	if err != nil {
		t.Fatalf("NewQdrantClient() error = %v", err)
	}
	if err := client.Upsert(context.Background(), []service.VectorPoint{{
		ID:     "point_1",
		Vector: []float32{1, 0},
		Payload: map[string]any{
			"document_id": "doc_1",
			"chunk_id":    "chunk_1",
		},
	}}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := client.DeleteByDocument(context.Background(), "doc_1"); err != nil {
		t.Fatalf("DeleteByDocument() error = %v", err)
	}
	hits, err := client.Search(context.Background(), service.VectorSearchRequest{
		Vector:           []float32{1, 0},
		KnowledgeBaseIDs: []string{"kb_1"},
		Tags:             []string{"policy"},
		MetadataFilter:   map[string]string{"plant": "A"},
		Limit:            5,
		ScoreThreshold:   0.2,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "point_1" || hits[0].Payload["chunk_id"] != "chunk_1" {
		t.Fatalf("hits = %+v", hits)
	}
	if !upsertCalled || !deleteCalled || !searchCalled {
		t.Fatalf("called upsert=%t delete=%t search=%t", upsertCalled, deleteCalled, searchCalled)
	}
}

func decodeRequest(t *testing.T, r *http.Request, target any) {
	t.Helper()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}
