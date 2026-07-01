package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type resourceRepositoryStub struct {
	activeQAConfig   QAConfigVersion
	savedInput       RetrievalTestInput
	savedRunErr      error
	saveCalled       bool
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
	return r.activeQAConfig, nil
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
func (r *resourceRepositoryStub) SaveRetrievalTestRun(_ context.Context, _ string, input RetrievalTestInput, results []RetrievalTestResult, duration time.Duration, runErr error) (RetrievalTestRun, error) {
	r.saveCalled = true
	r.savedInput = input
	r.savedRunErr = runErr
	status := "completed"
	errorMessage := ""
	if runErr != nil {
		status = "failed"
		errorMessage = "knowledge retrieval failed"
	}
	return RetrievalTestRun{ID: "rt-1", Question: input.Question, Status: status, ResultCount: len(results), LatencyMS: duration.Milliseconds(), ErrorMessage: errorMessage, Results: results}, nil
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

type knowledgeRetrieverStub struct {
	input   RetrievalTestInput
	results []RetrievalTestResult
	err     error
}

func (r *knowledgeRetrieverStub) Retrieve(_ context.Context, _ string, input RetrievalTestInput) ([]RetrievalTestResult, error) {
	r.input = input
	return r.results, r.err
}

type llmTesterStub struct{}

func (llmTesterStub) TestLLM(context.Context, RuntimeLLMConfig) (LLMConnectionTestResult, error) {
	return LLMConnectionTestResult{}, nil
}

type runCancellerStub struct{}

func (runCancellerStub) CancelActiveRun(string) {}

func TestCreateRetrievalTestRunMergesActiveConfigAndOverrides(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{
		ID:                      "qa-config-id",
		DefaultKnowledgeBaseIDs: []string{"kb-default"},
		Retrieval:               RetrievalSettings{TopK: 5, ScoreThreshold: .7, EnableRerank: false},
	}}
	retriever := &knowledgeRetrieverStub{results: []RetrievalTestResult{{DocumentID: "doc-1", Metadata: map[string]any{}}}}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	run, err := resources.CreateRetrievalTestRun(context.Background(), "user-1", RetrievalTestInput{
		Question:  "  what is qa  ",
		Retrieval: RetrievalSettings{TopK: 8},
		Overrides: RetrievalSettings{
			SimilarityThreshold: .35,
			UseRerank:           true,
			RerankTopN:          4,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantRetrieval := RetrievalSettings{TopK: 8, ScoreThreshold: .35, EnableRerank: true, RerankTopN: 4}
	if retriever.input.Question != "what is qa" || !reflect.DeepEqual(retriever.input.KnowledgeBaseIDs, []string{"kb-default"}) || retriever.input.QAConfigVersionID != "qa-config-id" || !reflect.DeepEqual(retriever.input.Retrieval, wantRetrieval) {
		t.Fatalf("retriever input=%+v", retriever.input)
	}
	if !repository.saveCalled || !reflect.DeepEqual(repository.savedInput.Retrieval, wantRetrieval) {
		t.Fatalf("saved input=%+v", repository.savedInput)
	}
	if run.Status != "completed" || run.ResultCount != 1 {
		t.Fatalf("run=%+v", run)
	}
}

func TestCreateRetrievalTestRunFallsBackToDefaultsAfterKnowledgeBaseNormalization(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{
		ID:                      "qa-config-id",
		DefaultKnowledgeBaseIDs: []string{" kb-default ", "kb-default"},
		Retrieval:               RetrievalSettings{TopK: 5, ScoreThreshold: .7},
	}}
	retriever := &knowledgeRetrieverStub{results: []RetrievalTestResult{{DocumentID: "doc-1", Metadata: map[string]any{}}}}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = resources.CreateRetrievalTestRun(context.Background(), "user-1", RetrievalTestInput{
		Question:         "query",
		KnowledgeBaseIDs: []string{" ", ""},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(retriever.input.KnowledgeBaseIDs, []string{"kb-default"}) {
		t.Fatalf("knowledgeBaseIds=%+v, want default knowledge base", retriever.input.KnowledgeBaseIDs)
	}
	if !reflect.DeepEqual(repository.savedInput.KnowledgeBaseIDs, []string{"kb-default"}) {
		t.Fatalf("saved knowledgeBaseIds=%+v, want default knowledge base", repository.savedInput.KnowledgeBaseIDs)
	}
}

func TestCreateRetrievalTestRunCanDisableActiveRerank(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{
		ID:        "qa-config-id",
		Retrieval: RetrievalSettings{TopK: 5, ScoreThreshold: .7, EnableRerank: true, RerankTopN: 3},
	}}
	retriever := &knowledgeRetrieverStub{results: []RetrievalTestResult{{DocumentID: "doc-1", Metadata: map[string]any{}}}}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	var input RetrievalTestInput
	if err := json.Unmarshal([]byte(`{"question":"query","overrides":{"enableRerank":false}}`), &input); err != nil {
		t.Fatal(err)
	}
	run, err := resources.CreateRetrievalTestRun(context.Background(), "user-1", input)
	if err != nil {
		t.Fatal(err)
	}

	wantRetrieval := RetrievalSettings{TopK: 5, ScoreThreshold: .7, EnableRerank: false, RerankTopN: 3}
	if !reflect.DeepEqual(retriever.input.Retrieval, wantRetrieval) {
		t.Fatalf("retrieval=%+v, want %+v", retriever.input.Retrieval, wantRetrieval)
	}
	if run.Status != "completed" {
		t.Fatalf("run=%+v", run)
	}
}

func TestCreateRetrievalTestRunCanClearActiveThresholds(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{
		ID:        "qa-config-id",
		Retrieval: RetrievalSettings{TopK: 5, ScoreThreshold: .7, RerankThreshold: .5},
	}}
	retriever := &knowledgeRetrieverStub{results: []RetrievalTestResult{{DocumentID: "doc-1", Metadata: map[string]any{}}}}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	var input RetrievalTestInput
	if err := json.Unmarshal([]byte(`{"question":"query","overrides":{"scoreThreshold":0,"rerankThreshold":0}}`), &input); err != nil {
		t.Fatal(err)
	}
	run, err := resources.CreateRetrievalTestRun(context.Background(), "user-1", input)
	if err != nil {
		t.Fatal(err)
	}

	wantRetrieval := RetrievalSettings{TopK: 5, scoreThresholdSet: true, rerankThresholdSet: true}
	if !reflect.DeepEqual(retriever.input.Retrieval, wantRetrieval) {
		t.Fatalf("retrieval=%+v, want %+v", retriever.input.Retrieval, wantRetrieval)
	}
	if run.Status != "completed" {
		t.Fatalf("run=%+v", run)
	}
}

func TestCreateRetrievalTestRunRejectsRerankTopNGreaterThanTopK(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{ID: "qa-config-id", Retrieval: RetrievalSettings{TopK: 5}}}
	retriever := &knowledgeRetrieverStub{results: []RetrievalTestResult{{DocumentID: "doc-1", Metadata: map[string]any{}}}}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = resources.CreateRetrievalTestRun(context.Background(), "user-1", RetrievalTestInput{
		Question:  "query",
		Overrides: RetrievalSettings{RerankTopN: 6},
	})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Fields["retrieval.rerankTopN"] == "" {
		t.Fatalf("error=%v, want retrieval.rerankTopN validation", err)
	}
	if repository.saveCalled || retriever.input.Question != "" {
		t.Fatalf("unexpected retrieval or save: input=%+v saveCalled=%v", retriever.input, repository.saveCalled)
	}
}

func TestCreateRetrievalTestRunSavesKnowledgeFailureSummary(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{ID: "qa-config-id", Retrieval: RetrievalSettings{TopK: 5}}}
	retriever := &knowledgeRetrieverStub{err: errors.New("dial tcp internal.service: timeout")}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	run, err := resources.CreateRetrievalTestRun(context.Background(), "user-1", RetrievalTestInput{Question: "query"})
	if err != nil {
		t.Fatalf("error=%v, want saved failed run", err)
	}
	if !repository.saveCalled || repository.savedRunErr == nil {
		t.Fatalf("saveCalled=%v runErr=%v", repository.saveCalled, repository.savedRunErr)
	}
	if run.Status != "failed" || run.ErrorMessage != "knowledge retrieval failed" {
		t.Fatalf("run=%+v", run)
	}
}

func TestCreateRetrievalTestRunPreservesKnowledgeForbidden(t *testing.T) {
	repository := &resourceRepositoryStub{activeQAConfig: QAConfigVersion{ID: "qa-config-id", Retrieval: RetrievalSettings{TopK: 5}}}
	retriever := &knowledgeRetrieverStub{err: NewError(CodeForbidden, "knowledge base access is forbidden", nil)}
	resources, err := NewResourceService(repository, retriever, llmTesterStub{}, RuntimeLLMConfig{}, runCancellerStub{})
	if err != nil {
		t.Fatal(err)
	}

	run, err := resources.CreateRetrievalTestRun(context.Background(), "user-1", RetrievalTestInput{Question: "query"})
	if err != nil {
		t.Fatalf("error=%v, want saved failed run", err)
	}
	if run.Status != "failed" || run.ErrorMessage != "knowledge retrieval failed" {
		t.Fatalf("run=%+v", run)
	}
}

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
	if citation.IsSourceAvailable || citation.Source == nil || citation.Source.Available {
		t.Fatalf("source should be unavailable (fail closed after check failure): %+v", citation)
	}
	if citation.Source.Reason != citationSourceUnavailableReason || citation.Source.DownloadEndpoint != "" {
		t.Fatalf("unexpected unavailable source mapping after check failure: %+v", citation.Source)
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
