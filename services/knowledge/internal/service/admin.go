package service

import (
	"context"
	"strings"
	"time"
)

type RuntimeConfig struct {
	EmbeddingProvider    string
	EmbeddingModel       string
	EmbeddingDimension   int
	QdrantCollection     string
	ParserBackend        string
	RerankProvider       string
	RerankModel          string
	RetrievalTopK        int
	ScoreThreshold       float64
	MaxConcurrentJobs    int
	ProcessingTimeoutSec int
	SecretRefs           map[string]string
}

type RuntimeConfigUpdate struct {
	ParserBackend        *string
	RerankProvider       *string
	RerankModel          *string
	RetrievalTopK        *int
	ScoreThreshold       *float64
	MaxConcurrentJobs    *int
	ProcessingTimeoutSec *int
	SecretRefs           map[string]string
}

type KnowledgeStats struct {
	KnowledgeBaseCount  int
	DocumentCount       int
	ChunkCount          int
	ReadyDocumentCount  int
	FailedDocumentCount int
	RecentUploads       []DailyUploadStat
}

type DailyUploadStat struct {
	Date  string
	Count int
}

type StatsFilter struct {
	OwnerUserID string
	Since       time.Time
	Until       time.Time
}

type StatsRepository interface {
	GetKnowledgeStats(ctx context.Context, filter StatsFilter) (KnowledgeStats, error)
}

func (s *KnowledgeService) GetRuntimeConfig(ctx context.Context, reqCtx RequestContext) (RuntimeConfig, error) {
	if err := validateActor(reqCtx); err != nil {
		return RuntimeConfig{}, err
	}
	if !canAdminKnowledge(reqCtx) {
		return RuntimeConfig{}, ForbiddenError("knowledge administration permission is required")
	}
	return s.getRuntimeConfig(), nil
}

func (s *KnowledgeService) UpdateRuntimeConfig(ctx context.Context, reqCtx RequestContext, update RuntimeConfigUpdate) (RuntimeConfig, error) {
	if err := validateActor(reqCtx); err != nil {
		return RuntimeConfig{}, err
	}
	if !canAdminKnowledge(reqCtx) {
		return RuntimeConfig{}, ForbiddenError("knowledge administration permission is required")
	}
	cfg := s.getRuntimeConfig()
	fields := map[string]string{}
	if update.ParserBackend != nil {
		cfg.ParserBackend = strings.TrimSpace(*update.ParserBackend)
	}
	if update.RerankProvider != nil {
		cfg.RerankProvider = strings.TrimSpace(*update.RerankProvider)
	}
	if update.RerankModel != nil {
		cfg.RerankModel = strings.TrimSpace(*update.RerankModel)
	}
	if update.RetrievalTopK != nil {
		if *update.RetrievalTopK < 1 || *update.RetrievalTopK > maxRetrievalTopK {
			fields["retrievalTopK"] = "must be between 1 and 100"
		} else {
			cfg.RetrievalTopK = *update.RetrievalTopK
		}
	}
	if update.ScoreThreshold != nil {
		if *update.ScoreThreshold < 0 {
			fields["scoreThreshold"] = "must be non-negative"
		} else {
			cfg.ScoreThreshold = *update.ScoreThreshold
		}
	}
	if update.MaxConcurrentJobs != nil {
		if *update.MaxConcurrentJobs < 1 || *update.MaxConcurrentJobs > 128 {
			fields["maxConcurrentJobs"] = "must be between 1 and 128"
		} else {
			cfg.MaxConcurrentJobs = *update.MaxConcurrentJobs
		}
	}
	if update.ProcessingTimeoutSec != nil {
		if *update.ProcessingTimeoutSec < 1 {
			fields["processingTimeoutSec"] = "must be positive"
		} else {
			cfg.ProcessingTimeoutSec = *update.ProcessingTimeoutSec
		}
	}
	if update.SecretRefs != nil {
		cfg.SecretRefs = normalizeSecretRefs(update.SecretRefs)
	}
	if len(fields) > 0 {
		return RuntimeConfig{}, ValidationError("request validation failed", fields)
	}
	s.setRuntimeConfig(cfg)
	return s.getRuntimeConfig(), nil
}

func (s *KnowledgeService) CreateReprocessingJob(ctx context.Context, reqCtx RequestContext, knowledgeBaseID string) (ProcessingJob, error) {
	if err := validateActor(reqCtx); err != nil {
		return ProcessingJob{}, err
	}
	base, err := s.GetKnowledgeBase(ctx, reqCtx, knowledgeBaseID)
	if err != nil {
		return ProcessingJob{}, err
	}
	if !canAdminKnowledge(reqCtx) && !hasPermission(reqCtx, "knowledge:write") {
		return ProcessingJob{}, ForbiddenError("knowledge write permission is required")
	}
	jobID, err := s.newID("job")
	if err != nil {
		return ProcessingJob{}, DependencyError("job id generation failed", err)
	}
	now := s.now().UTC()
	stage := JobStage("reprocessing")
	job := ProcessingJob{
		ID:              jobID,
		KnowledgeBaseID: base.ID,
		JobType:         JobTypeReprocess,
		Status:          JobStatusQueued,
		CurrentStage:    &stage,
		ProgressPercent: 0,
		Attempts:        0,
		MaxAttempts:     3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	created, err := s.repo.CreateProcessingJob(ctx, job)
	if err != nil {
		return ProcessingJob{}, mapJobRepositoryError(err, "job already exists", "job metadata write failed")
	}
	return created, nil
}

func (s *KnowledgeService) GetKnowledgeStats(ctx context.Context, reqCtx RequestContext) (KnowledgeStats, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeStats{}, err
	}
	if !canAdminKnowledge(reqCtx) {
		return KnowledgeStats{}, ForbiddenError("knowledge administration permission is required")
	}
	now := s.now().UTC()
	stats, err := s.repo.GetKnowledgeStats(ctx, StatsFilter{
		OwnerUserID: ownerFilter(reqCtx),
		Since:       startOfUTCDay(now.AddDate(0, 0, -29)),
		Until:       now,
	})
	if err != nil {
		return KnowledgeStats{}, DependencyError("knowledge statistics access failed", err)
	}
	stats.RecentUploads = normalizeRecentUploadStats(stats.RecentUploads, now, 30)
	return stats, nil
}

func (s *KnowledgeService) getRuntimeConfig() RuntimeConfig {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return cloneRuntimeConfig(s.runtimeConfig)
}

func (s *KnowledgeService) setRuntimeConfig(cfg RuntimeConfig) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.runtimeConfig = cloneRuntimeConfig(cfg)
}

func canAdminKnowledge(reqCtx RequestContext) bool {
	return hasPermission(reqCtx, "knowledge:admin") || hasPermission(reqCtx, "knowledge:write:any")
}

func normalizeSecretRefs(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	refs := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		refs[key] = value
	}
	return refs
}

func cloneRuntimeConfig(cfg RuntimeConfig) RuntimeConfig {
	cfg.SecretRefs = cloneStringMap(cfg.SecretRefs)
	return cfg
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

func normalizeRecentUploadStats(input []DailyUploadStat, now time.Time, days int) []DailyUploadStat {
	counts := make(map[string]int, len(input))
	for _, item := range input {
		if strings.TrimSpace(item.Date) == "" {
			continue
		}
		counts[item.Date] += item.Count
	}
	return recentUploadStats(counts, now, days)
}

func recentUploadStats(input map[string]int, now time.Time, days int) []DailyUploadStat {
	if days < 1 {
		days = 30
	}
	items := make([]DailyUploadStat, 0, days)
	start := now.UTC().AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		items = append(items, DailyUploadStat{Date: date, Count: input[date]})
	}
	return items
}

func startOfUTCDay(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
