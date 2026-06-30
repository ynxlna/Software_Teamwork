package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ResponseRun struct {
	ID                 string     `json:"id"`
	SessionID          string     `json:"sessionId"`
	UserMessageID      string     `json:"userMessageId"`
	AssistantMessageID string     `json:"assistantMessageId"`
	Status             string     `json:"status"`
	CurrentIteration   int        `json:"currentIteration"`
	MaxIterations      int        `json:"maxIterations"`
	TerminationReason  *string    `json:"terminationReason"`
	TotalTokens        int        `json:"totalTokens"`
	LatencyMS          int64      `json:"latencyMs"`
	CreatedAt          time.Time  `json:"createdAt"`
	CompletedAt        *time.Time `json:"completedAt"`
}

type StreamEvent struct {
	EventSeq  int            `json:"eventSeq"`
	EventType string         `json:"eventType"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
}

type Citation struct {
	ID                      string          `json:"id"`
	MessageID               string          `json:"messageId"`
	ResponseRunID           string          `json:"-"`
	CitationNo              int             `json:"citationNo"`
	DocumentID              string          `json:"documentId,omitempty"`
	DocID                   string          `json:"docId,omitempty"`
	DocumentName            string          `json:"documentName,omitempty"`
	DocName                 string          `json:"docName,omitempty"`
	KnowledgeBaseID         string          `json:"knowledgeBaseId,omitempty"`
	ChunkID                 string          `json:"chunkId,omitempty"`
	SectionPath             string          `json:"sectionPath,omitempty"`
	Text                    string          `json:"text,omitempty"`
	ContentPreview          string          `json:"contentPreview,omitempty"`
	Context                 string          `json:"context,omitempty"`
	PageNumber              *int            `json:"pageNumber,omitempty"`
	Score                   *float64        `json:"score,omitempty"`
	RerankScore             *float64        `json:"rerankScore,omitempty"`
	ChunkType               string          `json:"chunkType,omitempty"`
	IsSourceAvailable       bool            `json:"isSourceAvailable"`
	SourceUnavailableReason string          `json:"-"`
	Metadata                map[string]any  `json:"metadata"`
	Content                 string          `json:"content,omitempty"`
	Source                  *CitationSource `json:"source,omitempty"`
}

type CitationSource struct {
	Available        bool   `json:"available"`
	Reason           string `json:"reason,omitempty"`
	DownloadEndpoint string `json:"downloadEndpoint,omitempty"`
}

const citationSourceUnavailableReason = "source_deleted_or_forbidden"

func NormalizeCitation(item Citation) Citation {
	item.ID = strings.TrimSpace(item.ID)
	item.MessageID = strings.TrimSpace(item.MessageID)
	item.ResponseRunID = strings.TrimSpace(item.ResponseRunID)
	item.DocumentID = strings.TrimSpace(firstNonBlank(item.DocumentID, item.DocID))
	item.DocID = item.DocumentID
	item.DocumentName = strings.TrimSpace(firstNonBlank(item.DocumentName, item.DocName))
	item.DocName = item.DocumentName
	item.KnowledgeBaseID = strings.TrimSpace(item.KnowledgeBaseID)
	item.ChunkID = strings.TrimSpace(item.ChunkID)
	item.SectionPath = strings.TrimSpace(item.SectionPath)
	item.Text = strings.TrimSpace(item.Text)
	item.ContentPreview = strings.TrimSpace(firstNonBlank(item.ContentPreview, item.Text))
	item.Context = strings.TrimSpace(item.Context)
	item.ChunkType = strings.TrimSpace(item.ChunkType)
	item.SourceUnavailableReason = strings.TrimSpace(item.SourceUnavailableReason)
	item.Metadata = SanitizeCitationMetadata(item.Metadata)
	if item.Content == "" {
		item.Content = item.ContentPreview
	}
	if item.IsSourceAvailable && item.DocumentID == "" {
		item.IsSourceAvailable = false
	}
	item.Source = &CitationSource{Available: item.IsSourceAvailable}
	if item.IsSourceAvailable {
		item.Source.DownloadEndpoint = "/api/v1/documents/" + item.DocumentID + "/content"
	} else {
		if item.SourceUnavailableReason == "" {
			item.SourceUnavailableReason = citationSourceUnavailableReason
		}
		item.Source.Reason = item.SourceUnavailableReason
	}
	return item
}

func ApplyCitationSourceAvailability(item Citation, available bool) Citation {
	documentID := strings.TrimSpace(firstNonBlank(item.DocumentID, item.DocID))
	item.DocumentID = documentID
	item.DocID = documentID
	item.IsSourceAvailable = available && documentID != ""
	if item.IsSourceAvailable {
		item.SourceUnavailableReason = ""
	} else if strings.TrimSpace(item.SourceUnavailableReason) == "" {
		item.SourceUnavailableReason = citationSourceUnavailableReason
	}
	return NormalizeCitation(item)
}

func SanitizeCitationMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	cleaned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		if key == "" || isSensitiveCitationMetadataKey(key) {
			continue
		}
		if safe, ok := sanitizeCitationMetadataValue(value); ok {
			cleaned[key] = safe
		}
	}
	if cleaned == nil {
		return map[string]any{}
	}
	return cleaned
}

func sanitizeCitationMetadataValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil, string, bool, float64, float32, int, int64, int32, uint, uint64, uint32:
		return typed, true
	case map[string]any:
		return SanitizeCitationMetadata(typed), true
	case []any:
		values := make([]any, 0, len(typed))
		for _, item := range typed {
			if safe, ok := sanitizeCitationMetadataValue(item); ok {
				values = append(values, safe)
			}
		}
		return values, true
	default:
		return nil, false
	}
}

func isSensitiveCitationMetadataKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(strings.ToLower(key))
	for _, marker := range []string{
		"objectkey", "storagekey", "bucket", "internalurl", "signedurl", "presigned",
		"url", "vector", "embedding", "fileref", "fileid", "rawpayload", "payload",
		"prompt", "secret", "token", "apikey",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type AgentToolCall struct {
	ID                string         `json:"id"`
	ResponseRunID     string         `json:"responseRunId"`
	ModelInvocationID string         `json:"modelInvocationId,omitempty"`
	IterationNo       int            `json:"iterationNo,omitempty"`
	ToolCallID        string         `json:"toolCallId"`
	ToolName          string         `json:"toolName"`
	ArgumentsSummary  map[string]any `json:"argumentsSummary,omitempty"`
	ResultSummary     map[string]any `json:"resultSummary,omitempty"`
	Status            string         `json:"status"`
	LatencyMS         int64          `json:"latencyMs,omitempty"`
	StartedAt         time.Time      `json:"startedAt"`
	FinishedAt        *time.Time     `json:"finishedAt,omitempty"`
}

type ConfigKnowledgeBase struct {
	ID          string `json:"id"`
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	SortOrder   int    `json:"sortOrder"`
}

type AgentConfig struct {
	MaxIterations         int      `json:"maxIterations"`
	ToolTimeoutSeconds    int      `json:"toolTimeoutSeconds"`
	ModelTimeoutSeconds   int      `json:"modelTimeoutSeconds"`
	OverallTimeoutSeconds int      `json:"overallTimeoutSeconds"`
	EnabledToolNames      []string `json:"enabledToolNames"`
}

type QAConfigVersion struct {
	ID                      string                `json:"id"`
	VersionNo               int                   `json:"versionNo"`
	DefaultKnowledgeBaseIDs []string              `json:"defaultKnowledgeBaseIds"`
	KnowledgeBases          []ConfigKnowledgeBase `json:"knowledgeBases"`
	Retrieval               RetrievalSettings     `json:"retrieval"`
	Agent                   AgentConfig           `json:"agent"`
	MaxIterations           int                   `json:"maxIterations,omitempty"`
	ToolTimeoutSeconds      int                   `json:"toolTimeoutSeconds,omitempty"`
	ModelTimeoutSeconds     int                   `json:"modelTimeoutSeconds,omitempty"`
	OverallTimeoutSeconds   int                   `json:"overallTimeoutSeconds,omitempty"`
	EnabledToolNames        []string              `json:"enabledToolNames,omitempty"`
	IsActive                bool                  `json:"isActive"`
	CreatedAt               time.Time             `json:"createdAt"`
}

type CreateQAConfigVersionInput struct {
	DefaultKnowledgeBaseIDs []string              `json:"defaultKnowledgeBaseIds,omitempty"`
	KnowledgeBases          []ConfigKnowledgeBase `json:"knowledgeBases,omitempty"`
	Retrieval               RetrievalSettings     `json:"retrieval,omitempty"`
	TopK                    int                   `json:"topK,omitempty"`
	SimilarityThreshold     float64               `json:"similarityThreshold,omitempty"`
	UseRerank               bool                  `json:"useRerank,omitempty"`
	RerankThreshold         float64               `json:"rerankThreshold,omitempty"`
	RerankTopN              int                   `json:"rerankTopN,omitempty"`
	Agent                   AgentConfig           `json:"agent,omitempty"`
	MaxIterations           int                   `json:"maxIterations,omitempty"`
	ToolTimeoutSeconds      int                   `json:"toolTimeoutSeconds,omitempty"`
	ModelTimeoutSeconds     int                   `json:"modelTimeoutSeconds,omitempty"`
	OverallTimeoutSeconds   int                   `json:"overallTimeoutSeconds,omitempty"`
	EnabledToolNames        []string              `json:"enabledToolNames,omitempty"`
	Activate                *bool                 `json:"activate,omitempty"`
}

type LLMConfigVersion struct {
	ID             string    `json:"id"`
	VersionNo      int       `json:"versionNo"`
	Provider       string    `json:"provider"`
	ProfileID      string    `json:"profileId"`
	ModelName      string    `json:"modelName"`
	TimeoutSeconds int       `json:"timeoutSeconds"`
	Temperature    float64   `json:"temperature"`
	MaxTokens      int       `json:"maxTokens"`
	IsActive       bool      `json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
}

type CreateLLMConfigVersionInput struct {
	Provider       string  `json:"provider"`
	ProfileID      string  `json:"profileId"`
	ModelName      string  `json:"modelName"`
	TimeoutSeconds int     `json:"timeoutSeconds,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	MaxTokens      int     `json:"maxTokens,omitempty"`
	Activate       *bool   `json:"activate,omitempty"`
}

type LLMProfileTestInput struct {
	Provider       string `json:"provider"`
	ProfileID      string `json:"profileId"`
	ModelName      string `json:"modelName"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type LLMProfileTestResult struct {
	ID           string    `json:"id"`
	Success      bool      `json:"success"`
	LatencyMS    int64     `json:"latencyMs"`
	ModelName    string    `json:"modelName"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	TestedAt     time.Time `json:"testedAt"`
}

type RetrievalTestInput struct {
	Question         string            `json:"question"`
	Query            string            `json:"query,omitempty"`
	KnowledgeBaseIDs []string          `json:"knowledgeBaseIds,omitempty"`
	Retrieval        RetrievalSettings `json:"retrieval,omitempty"`
	Overrides        RetrievalSettings `json:"overrides,omitempty"`
}

type RetrievalTestResult struct {
	RankNo          int            `json:"rankNo"`
	KnowledgeBaseID string         `json:"knowledgeBaseId,omitempty"`
	DocumentID      string         `json:"documentId,omitempty"`
	DocumentName    string         `json:"documentName,omitempty"`
	ChunkID         string         `json:"chunkId,omitempty"`
	SectionPath     string         `json:"sectionPath,omitempty"`
	Score           float64        `json:"score,omitempty"`
	VectorScore     float64        `json:"vectorScore,omitempty"`
	RerankScore     *float64       `json:"rerankScore,omitempty"`
	ContentPreview  string         `json:"contentPreview,omitempty"`
	Metadata        map[string]any `json:"metadata"`
}

type RetrievalTestRun struct {
	ID         string                `json:"id"`
	Question   string                `json:"question"`
	Status     string                `json:"status"`
	Results    []RetrievalTestResult `json:"results"`
	CreatedAt  time.Time             `json:"createdAt"`
	FinishedAt *time.Time            `json:"finishedAt,omitempty"`
}

type MetricsOverview struct {
	TotalQACount       int   `json:"totalQaCount"`
	TodayQACount       int   `json:"todayQaCount"`
	TotalQuestionCount int   `json:"totalQuestionCount"`
	ConversationCount  int   `json:"conversationCount"`
	AvgLatencyMS       int64 `json:"avgLatencyMs"`
	ActiveUsersToday   int   `json:"activeUsersToday"`
}

type MetricsTrendPoint struct {
	Date          string `json:"date"`
	Count         int    `json:"count"`
	QuestionCount int    `json:"questionCount"`
}
type MetricsTrend struct {
	Days   int                 `json:"days"`
	Points []MetricsTrendPoint `json:"points"`
}
type TopQuery struct {
	Query        string    `json:"query"`
	Count        int       `json:"count"`
	AvgLatencyMS int64     `json:"avgLatencyMs"`
	LastAskedAt  time.Time `json:"lastAskedAt"`
}
type IntentDistribution struct {
	Intent  string  `json:"intent"`
	Label   string  `json:"label"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type ResourceRepository interface {
	GetResponseRun(context.Context, string, string) (ResponseRun, error)
	CancelResponseRun(context.Context, string, string) (ResponseRun, error)
	ListStreamEvents(context.Context, string, string, string, int) ([]StreamEvent, error)
	ListMessageCitations(context.Context, string, string) ([]Citation, error)
	GetCitation(context.Context, string, string) (Citation, error)
	LookupCitations(context.Context, string, []string) ([]Citation, error)
	ListToolCalls(context.Context, string, string) ([]AgentToolCall, error)
	GetActiveQAConfigVersion(context.Context) (QAConfigVersion, error)
	CreateQAConfigVersionResource(context.Context, string, CreateQAConfigVersionInput) (QAConfigVersion, error)
	GetActiveLLMConfigVersion(context.Context) (LLMConfigVersion, error)
	CreateLLMConfigVersionResource(context.Context, string, CreateLLMConfigVersionInput) (LLMConfigVersion, error)
	SaveLLMConnectionTest(context.Context, string, LLMProfileTestResult) (LLMProfileTestResult, error)
	SaveRetrievalTestRun(context.Context, string, RetrievalTestInput, []RetrievalTestResult, time.Duration, error) (RetrievalTestRun, error)
	GetRetrievalTestRun(context.Context, string, string) (RetrievalTestRun, error)
	GetMetricsOverview(context.Context, int) (MetricsOverview, error)
	GetMetricsTrend(context.Context, int) (MetricsTrend, error)
	GetTopQueries(context.Context, int, int) ([]TopQuery, error)
	GetIntentDistribution(context.Context, int) ([]IntentDistribution, error)
}

type KnowledgeRetriever interface {
	Retrieve(context.Context, string, RetrievalTestInput) ([]RetrievalTestResult, error)
}

type CitationSourceChecker interface {
	CheckCitationSources(context.Context, string, []string) (map[string]bool, error)
}

type ActiveRunCanceller interface{ CancelActiveRun(string) }

type ResourceService struct {
	repository    ResourceRepository
	retriever     KnowledgeRetriever
	sourceChecker CitationSourceChecker
	llmTester     LLMConnectionTester
	bootstrap     RuntimeLLMConfig
	canceller     ActiveRunCanceller
	now           func() time.Time
}

func NewResourceService(repository ResourceRepository, retriever KnowledgeRetriever, tester LLMConnectionTester, bootstrap RuntimeLLMConfig, canceller ActiveRunCanceller) (*ResourceService, error) {
	if repository == nil || retriever == nil || tester == nil || canceller == nil {
		return nil, errors.New("resource repository, retriever, LLM tester and run canceller are required")
	}
	sourceChecker, _ := retriever.(CitationSourceChecker)
	return &ResourceService{repository: repository, retriever: retriever, sourceChecker: sourceChecker, llmTester: tester, bootstrap: bootstrap, canceller: canceller, now: time.Now}, nil
}

func (s *ResourceService) GetResponseRun(ctx context.Context, userID, id string) (ResponseRun, error) {
	return s.repository.GetResponseRun(ctx, userID, id)
}
func (s *ResourceService) CancelResponseRun(ctx context.Context, userID, id string) (ResponseRun, error) {
	run, err := s.repository.CancelResponseRun(ctx, userID, id)
	if err == nil {
		s.canceller.CancelActiveRun(id)
	}
	return run, err
}
func (s *ResourceService) ListStreamEvents(ctx context.Context, userID, sessionID, runID string, after int) ([]StreamEvent, error) {
	return s.repository.ListStreamEvents(ctx, userID, sessionID, runID, after)
}
func (s *ResourceService) ListMessageCitations(ctx context.Context, userID, id string) ([]Citation, error) {
	items, err := s.repository.ListMessageCitations(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return revalidateCitationSources(ctx, userID, s.sourceChecker, items), nil
}
func (s *ResourceService) GetCitation(ctx context.Context, userID, id string) (Citation, error) {
	item, err := s.repository.GetCitation(ctx, userID, id)
	if err != nil {
		return Citation{}, err
	}
	items := revalidateCitationSources(ctx, userID, s.sourceChecker, []Citation{item})
	return items[0], nil
}
func (s *ResourceService) LookupCitations(ctx context.Context, userID string, ids []string) ([]Citation, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, ValidationError(map[string]string{"citationIds": "must contain between 1 and 100 items"})
	}
	items, err := s.repository.LookupCitations(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	return revalidateCitationSources(ctx, userID, s.sourceChecker, items), nil
}

func revalidateCitationSources(ctx context.Context, userID string, sourceChecker CitationSourceChecker, items []Citation) []Citation {
	if len(items) == 0 {
		return items
	}
	documentIDs := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		documentID := strings.TrimSpace(firstNonBlank(item.DocumentID, item.DocID))
		if documentID == "" {
			continue
		}
		if _, ok := seen[documentID]; ok {
			continue
		}
		seen[documentID] = struct{}{}
		documentIDs = append(documentIDs, documentID)
	}
	checkedOK := false
	availability := map[string]bool{}
	if sourceChecker != nil && len(documentIDs) > 0 {
		checked, err := sourceChecker.CheckCitationSources(ctx, userID, documentIDs)
		if err == nil && checked != nil {
			availability = checked
			checkedOK = true
		}
	}
	normalized := make([]Citation, 0, len(items))
	for _, item := range items {
		documentID := strings.TrimSpace(firstNonBlank(item.DocumentID, item.DocID))
		if checkedOK && documentID != "" {
			normalized = append(normalized, ApplyCitationSourceAvailability(item, availability[documentID]))
		} else if !checkedOK {
			normalized = append(normalized, NormalizeCitation(item))
		} else {
			normalized = append(normalized, NormalizeCitation(item))
		}
	}
	return normalized
}

func revalidateMessageCitations(ctx context.Context, userID string, sourceChecker CitationSourceChecker, messages []Message) {
	if len(messages) == 0 {
		return
	}
	type messageCitationIndex struct {
		message  int
		citation int
	}
	indexes := make([]messageCitationIndex, 0)
	items := make([]Citation, 0)
	for messageIndex := range messages {
		for citationIndex := range messages[messageIndex].Citations {
			indexes = append(indexes, messageCitationIndex{message: messageIndex, citation: citationIndex})
			items = append(items, messages[messageIndex].Citations[citationIndex])
		}
	}
	if len(items) == 0 {
		return
	}
	normalized := revalidateCitationSources(ctx, userID, sourceChecker, items)
	for index, item := range normalized {
		target := indexes[index]
		messages[target.message].Citations[target.citation] = item
	}
}

func (s *ResourceService) ListToolCalls(ctx context.Context, userID, id string) ([]AgentToolCall, error) {
	return s.repository.ListToolCalls(ctx, userID, id)
}
func (s *ResourceService) GetActiveQAConfigVersion(ctx context.Context) (QAConfigVersion, error) {
	return s.repository.GetActiveQAConfigVersion(ctx)
}
func (s *ResourceService) CreateQAConfigVersion(ctx context.Context, userID string, input CreateQAConfigVersionInput) (QAConfigVersion, error) {
	fields := map[string]string{}
	topK := input.Retrieval.TopK
	if topK == 0 {
		topK = input.TopK
	}
	if topK < 0 || topK > 100 {
		fields["retrieval.topK"] = "must be between 1 and 100"
	}
	threshold := input.Retrieval.ScoreThreshold
	if threshold == 0 {
		threshold = input.SimilarityThreshold
	}
	if threshold < 0 || threshold > 1 {
		fields["retrieval.scoreThreshold"] = "must be between 0 and 1"
	}
	for name, value := range map[string]int{"agent.maxIterations": max(input.Agent.MaxIterations, input.MaxIterations), "agent.toolTimeoutSeconds": max(input.Agent.ToolTimeoutSeconds, input.ToolTimeoutSeconds), "agent.modelTimeoutSeconds": max(input.Agent.ModelTimeoutSeconds, input.ModelTimeoutSeconds), "agent.overallTimeoutSeconds": max(input.Agent.OverallTimeoutSeconds, input.OverallTimeoutSeconds)} {
		if value < 0 {
			fields[name] = "must be positive"
		}
	}
	if len(input.KnowledgeBases) > 50 || len(input.DefaultKnowledgeBaseIDs) > 50 {
		fields["knowledgeBases"] = "must not contain more than 50 items"
	}
	if len(fields) > 0 {
		return QAConfigVersion{}, ValidationError(fields)
	}
	return s.repository.CreateQAConfigVersionResource(ctx, userID, input)
}
func (s *ResourceService) GetActiveLLMConfigVersion(ctx context.Context) (LLMConfigVersion, error) {
	return s.repository.GetActiveLLMConfigVersion(ctx)
}
func (s *ResourceService) CreateLLMConfigVersion(ctx context.Context, userID string, input CreateLLMConfigVersionInput) (LLMConfigVersion, error) {
	if fields := validateLLMProfile(input.Provider, input.ProfileID, input.ModelName, input.TimeoutSeconds, input.Temperature, input.MaxTokens); len(fields) > 0 {
		return LLMConfigVersion{}, ValidationError(fields)
	}
	return s.repository.CreateLLMConfigVersionResource(ctx, userID, input)
}
func (s *ResourceService) TestLLMConnection(ctx context.Context, userID string, input LLMProfileTestInput) (LLMProfileTestResult, error) {
	if fields := validateLLMProfile(input.Provider, input.ProfileID, input.ModelName, input.TimeoutSeconds, 0, 0); len(fields) > 0 {
		return LLMProfileTestResult{}, ValidationError(fields)
	}
	runtime := s.bootstrap
	runtime.ProfileID = input.ProfileID
	runtime.Model = input.ModelName
	if input.TimeoutSeconds > 0 {
		runtime.Timeout = time.Duration(input.TimeoutSeconds) * time.Second
	}
	started := s.now()
	tested, err := s.llmTester.TestLLM(ctx, runtime)
	result := LLMProfileTestResult{ID: newID("test"), Success: err == nil && tested.Success, LatencyMS: time.Since(started).Milliseconds(), ModelName: input.ModelName, TestedAt: s.now().UTC()}
	if err != nil {
		result.ErrorCode = "dependency_error"
		result.ErrorMessage = "AI Gateway connection test failed"
	}
	return s.repository.SaveLLMConnectionTest(ctx, userID, result)
}
func (s *ResourceService) CreateRetrievalTestRun(ctx context.Context, userID string, input RetrievalTestInput) (RetrievalTestRun, error) {
	if strings.TrimSpace(input.Question) == "" {
		input.Question = strings.TrimSpace(input.Query)
	}
	if input.Question == "" {
		return RetrievalTestRun{}, ValidationError(map[string]string{"question": "is required"})
	}
	started := s.now()
	results, retrieveErr := s.retriever.Retrieve(ctx, userID, input)
	run, saveErr := s.repository.SaveRetrievalTestRun(context.WithoutCancel(ctx), userID, input, results, s.now().Sub(started), retrieveErr)
	if saveErr != nil {
		return RetrievalTestRun{}, saveErr
	}
	if retrieveErr != nil {
		return run, NewError(CodeDependency, "knowledge retrieval failed", retrieveErr)
	}
	return run, nil
}
func (s *ResourceService) GetRetrievalTestRun(ctx context.Context, userID, id string) (RetrievalTestRun, error) {
	return s.repository.GetRetrievalTestRun(ctx, userID, id)
}
func (s *ResourceService) GetMetricsOverview(ctx context.Context, days int) (MetricsOverview, error) {
	return s.repository.GetMetricsOverview(ctx, days)
}
func (s *ResourceService) GetMetricsTrend(ctx context.Context, days int) (MetricsTrend, error) {
	return s.repository.GetMetricsTrend(ctx, days)
}
func (s *ResourceService) GetTopQueries(ctx context.Context, days, limit int) ([]TopQuery, error) {
	return s.repository.GetTopQueries(ctx, days, limit)
}
func (s *ResourceService) GetIntentDistribution(ctx context.Context, days int) ([]IntentDistribution, error) {
	return s.repository.GetIntentDistribution(ctx, days)
}

func validateLLMProfile(provider, profileID, model string, timeout int, temperature float64, maxTokens int) map[string]string {
	fields := map[string]string{}
	if provider != "ai-gateway" {
		fields["provider"] = "must be ai-gateway"
	}
	if strings.TrimSpace(profileID) == "" {
		fields["profileId"] = "is required"
	}
	if strings.TrimSpace(model) == "" {
		fields["modelName"] = "is required"
	}
	if timeout < 0 {
		fields["timeoutSeconds"] = "must be positive"
	}
	if temperature < 0 || temperature > 2 {
		fields["temperature"] = "must be between 0 and 2"
	}
	if maxTokens < 0 {
		fields["maxTokens"] = "must be positive"
	}
	return fields
}
