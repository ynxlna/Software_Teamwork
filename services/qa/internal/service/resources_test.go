package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestResourceServiceRevalidatesCitationSourceAvailability(t *testing.T) {
	repository := &resourceRepositoryStub{
		messageCitations: []Citation{{
			ID:                "citation-1",
			MessageID:         "message-1",
			CitationNo:        1,
			DocumentID:        "doc-1",
			DocumentName:      "Manual",
			Text:              "saved quote",
			ContentPreview:    "saved preview",
			IsSourceAvailable: true,
			Metadata:          map[string]any{"pageLabel": "1"},
		}},
	}
	retriever := &resourceRetrieverStub{availability: map[string]bool{"doc-1": true}}
	resources, err := NewResourceService(repository, retriever, resourceLLMTester{}, RuntimeLLMConfig{}, resourceCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}
	citations, err := resources.ListMessageCitations(context.Background(), "user-1", "message-1")
	if err != nil {
		t.Fatal(err)
	}
	if retriever.userID != "user-1" || !reflect.DeepEqual(retriever.documentIDs, []string{"doc-1"}) {
		t.Fatalf("source check user=%q ids=%+v", retriever.userID, retriever.documentIDs)
	}
	if len(citations) != 1 {
		t.Fatalf("citations=%+v", citations)
	}
	citation := citations[0]
	if !citation.IsSourceAvailable || citation.Source == nil || !citation.Source.Available {
		t.Fatalf("source should be available: %+v", citation)
	}
	if citation.Source.DownloadEndpoint != "/api/v1/documents/doc-1/content" || citation.Source.Reason != "" {
		t.Fatalf("unexpected source mapping: %+v", citation.Source)
	}
}

func TestResourceServicePreservesSnapshotWhenSourceCheckFails(t *testing.T) {
	repository := &resourceRepositoryStub{
		citation: Citation{
			ID:                "citation-1",
			MessageID:         "message-1",
			CitationNo:        1,
			DocumentID:        "doc-1",
			DocumentName:      "Manual",
			Text:              "saved quote",
			ContentPreview:    "saved preview",
			Context:           "saved context",
			IsSourceAvailable: true,
			Metadata:          map[string]any{"pageLabel": "1", "objectKey": "secret"},
		},
	}
	retriever := &resourceRetrieverStub{checkErr: errors.New("knowledge unavailable")}
	resources, err := NewResourceService(repository, retriever, resourceLLMTester{}, RuntimeLLMConfig{}, resourceCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}
	citation, err := resources.GetCitation(context.Background(), "user-1", "citation-1")
	if err != nil {
		t.Fatal(err)
	}
	if !citation.IsSourceAvailable || citation.Source == nil || !citation.Source.Available {
		t.Fatalf("source should be available (snapshot preserved after check failure): %+v", citation)
	}
	if citation.Source.Reason != "" || citation.Source.DownloadEndpoint != "/api/v1/documents/doc-1/content" {
		t.Fatalf("unexpected source mapping after check failure: %+v", citation.Source)
	}
	if citation.Text != "saved quote" || citation.ContentPreview != "saved preview" || citation.Content != "saved preview" {
		t.Fatalf("snapshot text was not preserved: %+v", citation)
	}
	if citation.Metadata["pageLabel"] != "1" {
		t.Fatalf("safe metadata was not preserved: %#v", citation.Metadata)
	}
	if _, ok := citation.Metadata["objectKey"]; ok {
		t.Fatalf("sensitive metadata leaked: %#v", citation.Metadata)
	}
}

type resourceRepositoryStub struct {
	messageCitations []Citation
	citation         Citation
	lookupCitations  []Citation
}

func (r *resourceRepositoryStub) GetResponseRun(context.Context, string, string) (ResponseRun, error) {
	return ResponseRun{}, nil
}
func (r *resourceRepositoryStub) CancelResponseRun(context.Context, string, string) (ResponseRun, error) {
	return ResponseRun{}, nil
}
func (r *resourceRepositoryStub) ListStreamEvents(context.Context, string, string, string, int) ([]StreamEvent, error) {
	return nil, nil
}
func (r *resourceRepositoryStub) ListMessageCitations(context.Context, string, string) ([]Citation, error) {
	return append([]Citation(nil), r.messageCitations...), nil
}
func (r *resourceRepositoryStub) GetCitation(context.Context, string, string) (Citation, error) {
	return r.citation, nil
}
func (r *resourceRepositoryStub) LookupCitations(context.Context, string, []string) ([]Citation, error) {
	return append([]Citation(nil), r.lookupCitations...), nil
}
func (r *resourceRepositoryStub) ListToolCalls(context.Context, string, string) ([]AgentToolCall, error) {
	return nil, nil
}
func (r *resourceRepositoryStub) GetActiveQAConfigVersion(context.Context) (QAConfigVersion, error) {
	return QAConfigVersion{}, nil
}
func (r *resourceRepositoryStub) CreateQAConfigVersionResource(context.Context, string, CreateQAConfigVersionInput) (QAConfigVersion, error) {
	return QAConfigVersion{}, nil
}
func (r *resourceRepositoryStub) GetActiveLLMConfigVersion(context.Context) (LLMConfigVersion, error) {
	return LLMConfigVersion{}, nil
}
func (r *resourceRepositoryStub) CreateLLMConfigVersionResource(context.Context, string, CreateLLMConfigVersionInput) (LLMConfigVersion, error) {
	return LLMConfigVersion{}, nil
}
func (r *resourceRepositoryStub) SaveLLMConnectionTest(context.Context, string, LLMProfileTestResult) (LLMProfileTestResult, error) {
	return LLMProfileTestResult{}, nil
}
func (r *resourceRepositoryStub) SaveRetrievalTestRun(context.Context, string, RetrievalTestInput, []RetrievalTestResult, time.Duration, error) (RetrievalTestRun, error) {
	return RetrievalTestRun{}, nil
}
func (r *resourceRepositoryStub) GetRetrievalTestRun(context.Context, string, string) (RetrievalTestRun, error) {
	return RetrievalTestRun{}, nil
}
func (r *resourceRepositoryStub) GetMetricsOverview(context.Context, int) (MetricsOverview, error) {
	return MetricsOverview{}, nil
}
func (r *resourceRepositoryStub) GetMetricsTrend(context.Context, int) (MetricsTrend, error) {
	return MetricsTrend{}, nil
}
func (r *resourceRepositoryStub) GetTopQueries(context.Context, int, int) ([]TopQuery, error) {
	return nil, nil
}
func (r *resourceRepositoryStub) GetIntentDistribution(context.Context, int) ([]IntentDistribution, error) {
	return nil, nil
}

type resourceRetrieverStub struct {
	availability map[string]bool
	checkErr     error
	userID       string
	documentIDs  []string
}

func (r *resourceRetrieverStub) Retrieve(context.Context, string, RetrievalTestInput) ([]RetrievalTestResult, error) {
	return nil, nil
}

func (r *resourceRetrieverStub) CheckCitationSources(_ context.Context, userID string, documentIDs []string) (map[string]bool, error) {
	r.userID = userID
	r.documentIDs = append([]string(nil), documentIDs...)
	return r.availability, r.checkErr
}

type resourceLLMTester struct{}

func (resourceLLMTester) TestLLM(context.Context, RuntimeLLMConfig) (LLMConnectionTestResult, error) {
	return LLMConnectionTestResult{Success: true}, nil
}

type resourceCancellerStub struct{}

func (resourceCancellerStub) CancelActiveRun(string) {}
