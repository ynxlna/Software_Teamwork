package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type MemoryRepository struct {
	mu             sync.RWMutex
	knowledgeBases map[string]service.KnowledgeBase
	documents      map[string]service.KnowledgeDocument
	jobs           map[string]service.ProcessingJob
	parserConfigs  map[string]service.ParserConfig
	parserAudits   []service.ParserConfigAudit
	chunks         map[string]service.DocumentChunk
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		knowledgeBases: map[string]service.KnowledgeBase{},
		documents:      map[string]service.KnowledgeDocument{},
		jobs:           map[string]service.ProcessingJob{},
		parserConfigs:  map[string]service.ParserConfig{},
		chunks:         map[string]service.DocumentChunk{},
	}
}

func (r *MemoryRepository) ListParserConfigs(ctx context.Context, enabled *bool) ([]service.ParserConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]service.ParserConfig, 0, len(r.parserConfigs))
	for _, config := range r.parserConfigs {
		if config.DeletedAt != nil || enabled != nil && config.Enabled != *enabled {
			continue
		}
		items = append(items, cloneParserConfig(config))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (r *MemoryRepository) GetParserConfig(ctx context.Context, id string) (service.ParserConfig, error) {
	if err := ctx.Err(); err != nil {
		return service.ParserConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	config, ok := r.parserConfigs[id]
	if !ok || config.DeletedAt != nil {
		return service.ParserConfig{}, service.ErrNotFound
	}
	return cloneParserConfig(config), nil
}

func (r *MemoryRepository) CreateParserConfig(ctx context.Context, config service.ParserConfig, audit service.ParserConfigAudit) (service.ParserConfig, error) {
	if err := ctx.Err(); err != nil {
		return service.ParserConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.parserConfigs {
		if existing.DeletedAt == nil && strings.EqualFold(existing.Name, config.Name) {
			return service.ParserConfig{}, service.ErrConflict
		}
	}
	if config.IsDefault {
		r.clearDefaultLocked(config.ID, config.UpdatedAt)
	}
	r.parserConfigs[config.ID] = cloneParserConfig(config)
	r.parserAudits = append(r.parserAudits, cloneParserAudit(audit))
	return cloneParserConfig(config), nil
}

func (r *MemoryRepository) UpdateParserConfig(ctx context.Context, config service.ParserConfig, audit service.ParserConfigAudit) (service.ParserConfig, error) {
	if err := ctx.Err(); err != nil {
		return service.ParserConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.parserConfigs[config.ID]; !ok || existing.DeletedAt != nil {
		return service.ParserConfig{}, service.ErrNotFound
	}
	for id, existing := range r.parserConfigs {
		if id != config.ID && existing.DeletedAt == nil && strings.EqualFold(existing.Name, config.Name) {
			return service.ParserConfig{}, service.ErrConflict
		}
	}
	if config.IsDefault {
		r.clearDefaultLocked(config.ID, config.UpdatedAt)
	}
	r.parserConfigs[config.ID] = cloneParserConfig(config)
	r.parserAudits = append(r.parserAudits, cloneParserAudit(audit))
	return cloneParserConfig(config), nil
}

func (r *MemoryRepository) SoftDeleteParserConfig(ctx context.Context, id string, deletedAt time.Time, audit service.ParserConfigAudit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	config, ok := r.parserConfigs[id]
	if !ok || config.DeletedAt != nil {
		return service.ErrNotFound
	}
	if config.IsDefault {
		return service.ErrConflict
	}
	config.Enabled = false
	config.UpdatedAt = deletedAt
	config.DeletedAt = &deletedAt
	r.parserConfigs[id] = config
	r.parserAudits = append(r.parserAudits, cloneParserAudit(audit))
	return nil
}

func (r *MemoryRepository) GetEffectiveParserConfig(ctx context.Context, contentType string) (service.ParserConfig, error) {
	if err := ctx.Err(); err != nil {
		return service.ParserConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	type candidate struct {
		config service.ParserConfig
		rank   int
	}
	candidates := []candidate{}
	for _, config := range r.parserConfigs {
		if config.DeletedAt != nil || !config.Enabled {
			continue
		}
		rank, ok := parserContentTypeMatchRank(config, contentType)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{config: config, rank: rank})
	}
	if len(candidates) == 0 {
		return service.ParserConfig{}, service.ErrNotFound
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.rank != right.rank {
			return left.rank < right.rank
		}
		if left.config.IsDefault != right.config.IsDefault {
			return left.config.IsDefault
		}
		if !left.config.CreatedAt.Equal(right.config.CreatedAt) {
			return left.config.CreatedAt.Before(right.config.CreatedAt)
		}
		return left.config.ID < right.config.ID
	})
	return cloneParserConfig(candidates[0].config), nil
}

func (r *MemoryRepository) SeedParserConfig(config service.ParserConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parserConfigs[config.ID] = cloneParserConfig(config)
}
func (r *MemoryRepository) ParserAudits() []service.ParserConfigAudit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]service.ParserConfigAudit, len(r.parserAudits))
	for i, v := range r.parserAudits {
		out[i] = cloneParserAudit(v)
	}
	return out
}
func (r *MemoryRepository) clearDefaultLocked(except string, updatedAt time.Time) {
	for id, config := range r.parserConfigs {
		if id != except && config.DeletedAt == nil && config.IsDefault {
			config.IsDefault = false
			config.UpdatedAt = updatedAt
			r.parserConfigs[id] = config
		}
	}
}
func parserContentTypeMatchRank(config service.ParserConfig, contentType string) (int, bool) {
	if contentType == "" {
		return 0, true
	}
	if len(config.SupportedContentTypes) == 0 {
		return 2, true
	}
	wildcardMatch := false
	for _, candidate := range config.SupportedContentTypes {
		if candidate == contentType {
			return 0, true
		}
		if strings.HasSuffix(candidate, "/*") && strings.HasPrefix(contentType, strings.TrimSuffix(candidate, "*")) {
			wildcardMatch = true
		}
	}
	if wildcardMatch {
		return 1, true
	}
	return 0, false
}
func cloneParserConfig(config service.ParserConfig) service.ParserConfig {
	config.SupportedContentTypes = append([]string(nil), config.SupportedContentTypes...)
	config.DefaultParameters = cloneRaw(config.DefaultParameters)
	config.EndpointURL = cloneStringPtr(config.EndpointURL)
	if config.DeletedAt != nil {
		v := *config.DeletedAt
		config.DeletedAt = &v
	}
	return config
}
func cloneParserAudit(audit service.ParserConfigAudit) service.ParserConfigAudit {
	audit.Summary = cloneRaw(audit.Summary)
	return audit
}

func (r *MemoryRepository) CreateKnowledgeBase(ctx context.Context, input service.CreateKnowledgeBaseRecord) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.knowledgeBases[input.ID]; exists {
		return service.KnowledgeBase{}, service.ErrConflict
	}
	kb := service.KnowledgeBase{
		ID:                input.ID,
		Name:              input.Name,
		Description:       input.Description,
		DocType:           input.DocType,
		ChunkStrategy:     cloneRaw(input.ChunkStrategy),
		RetrievalStrategy: cloneRaw(input.RetrievalStrategy),
		CreatedBy:         input.CreatedBy,
		CreatedAt:         input.CreatedAt,
		UpdatedAt:         input.UpdatedAt,
	}
	r.knowledgeBases[kb.ID] = kb
	return r.hydrateKnowledgeBaseLocked(kb), nil
}

func (r *MemoryRepository) ListKnowledgeBases(ctx context.Context, scope service.AccessScope, page service.PageInput) (service.KnowledgeBaseList, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBaseList{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]service.KnowledgeBase, 0, len(r.knowledgeBases))
	for _, kb := range r.knowledgeBases {
		if kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
			continue
		}
		items = append(items, r.hydrateKnowledgeBaseLocked(kb))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := int64(len(items))
	items = paginate(items, page)
	return service.KnowledgeBaseList{
		Items: cloneKnowledgeBases(items),
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) GetKnowledgeBase(ctx context.Context, id string, scope service.AccessScope) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	kb, exists := r.knowledgeBases[id]
	if !exists || kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
		return service.KnowledgeBase{}, service.ErrNotFound
	}
	return cloneKnowledgeBase(r.hydrateKnowledgeBaseLocked(kb)), nil
}

func (r *MemoryRepository) UpdateKnowledgeBase(ctx context.Context, input service.UpdateKnowledgeBaseRecord, scope service.AccessScope) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	kb, exists := r.knowledgeBases[input.ID]
	if !exists || kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
		return service.KnowledgeBase{}, service.ErrNotFound
	}
	if input.Name != nil {
		kb.Name = *input.Name
	}
	if input.Description != nil {
		kb.Description = *input.Description
	}
	if input.DocType != nil {
		kb.DocType = *input.DocType
	}
	if input.ChunkStrategy != nil {
		kb.ChunkStrategy = cloneRaw(*input.ChunkStrategy)
	}
	if input.RetrievalStrategy != nil {
		kb.RetrievalStrategy = cloneRaw(*input.RetrievalStrategy)
	}
	kb.UpdatedAt = input.UpdatedAt
	r.knowledgeBases[kb.ID] = kb
	return cloneKnowledgeBase(r.hydrateKnowledgeBaseLocked(kb)), nil
}

func (r *MemoryRepository) SoftDeleteKnowledgeBase(ctx context.Context, id string, deletedAt time.Time, scope service.AccessScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	kb, exists := r.knowledgeBases[id]
	if !exists || kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
		return service.ErrNotFound
	}
	deleted := deletedAt.UTC()
	kb.DeletedAt = &deleted
	kb.UpdatedAt = deleted
	r.knowledgeBases[id] = kb

	for docID, doc := range r.documents {
		if doc.KnowledgeBaseID == id && doc.DeletedAt == nil {
			doc.DeletedAt = &deleted
			doc.UpdatedAt = deleted
			r.documents[docID] = doc
		}
	}
	return nil
}

func (r *MemoryRepository) CreateDocumentWithJob(ctx context.Context, input service.CreateDocumentWithJobRecord, scope service.AccessScope) (service.KnowledgeDocument, service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	kb, exists := r.knowledgeBases[input.KnowledgeBaseID]
	if !exists || kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrNotFound
	}
	if _, exists := r.documents[input.DocumentID]; exists {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
	}
	if _, exists := r.jobs[input.JobID]; exists {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
	}

	fileRef := input.FileRef
	contentType := input.ContentType
	sizeBytes := input.SizeBytes
	jobID := input.CurrentJobID
	stage := input.JobStage
	message := input.JobMessage
	doc := service.KnowledgeDocument{
		ID:              input.DocumentID,
		KnowledgeBaseID: input.KnowledgeBaseID,
		FileRef:         &fileRef,
		Name:            input.Name,
		ContentType:     &contentType,
		SizeBytes:       &sizeBytes,
		Status:          input.Status,
		Tags:            append([]string(nil), input.Tags...),
		CurrentJobID:    &jobID,
		CreatedBy:       input.CreatedBy,
		CreatedAt:       input.CreatedAt,
		UpdatedAt:       input.UpdatedAt,
	}
	documentID := input.DocumentID
	job := service.ProcessingJob{
		ID:                   input.JobID,
		KnowledgeBaseID:      input.KnowledgeBaseID,
		DocumentID:           &documentID,
		JobType:              input.JobType,
		Status:               input.JobStatus,
		CurrentStage:         &stage,
		ProgressPercent:      0,
		Message:              &message,
		Attempts:             0,
		MaxAttempts:          input.MaxAttempts,
		ParserConfigSnapshot: cloneRaw(input.ParserConfigSnapshot),
		CreatedAt:            input.CreatedAt,
		UpdatedAt:            input.UpdatedAt,
	}
	if input.ParserConfigID != "" {
		parserConfigID := input.ParserConfigID
		job.ParserConfigID = &parserConfigID
	}
	r.documents[doc.ID] = doc
	r.jobs[job.ID] = job
	return cloneDocument(r.hydrateDocumentLocked(doc)), cloneJob(job), nil
}

func (r *MemoryRepository) MarkDocumentJobFailed(ctx context.Context, documentID string, jobID string, expectedAttempts *int32, code string, message string, failedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, docExists := r.documents[documentID]
	job, jobExists := r.jobs[jobID]
	if !jobExists {
		return service.ErrNotFound
	}
	if expectedAttempts != nil && (job.Attempts != *expectedAttempts || job.Status != service.JobStatusRunning) {
		return service.ErrConflict
	}
	job.Status = service.JobStatusFailed
	job.ErrorCode = cloneStringPtr(&code)
	job.ErrorMessage = cloneStringPtr(&message)
	job.FinishedAt = &failedAt
	job.UpdatedAt = failedAt
	r.jobs[jobID] = job

	if docExists && doc.DeletedAt == nil {
		doc.Status = service.DocumentStatusFailed
		doc.ErrorCode = cloneStringPtr(&code)
		doc.ErrorMessage = cloneStringPtr(&message)
		doc.UpdatedAt = failedAt
		r.documents[documentID] = doc
	}
	return nil
}

func (r *MemoryRepository) GetProcessingJob(ctx context.Context, id string) (service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.ProcessingJob{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[id]
	if !exists {
		return service.ProcessingJob{}, service.ErrNotFound
	}
	return cloneJob(job), nil
}

func (r *MemoryRepository) UpdateJobState(ctx context.Context, id string, update service.JobStateUpdate) (service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.ProcessingJob{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[id]
	if !exists {
		return service.ProcessingJob{}, service.ErrNotFound
	}
	if update.ExpectedAttempts != nil && (job.Attempts != *update.ExpectedAttempts || job.Status != service.JobStatusRunning) {
		return service.ProcessingJob{}, service.ErrConflict
	}
	job.Status = update.Status
	job.ProgressPercent = update.ProgressPercent
	job.CurrentStage = cloneStringPtr(update.CurrentStage)
	job.Message = cloneStringPtr(update.Message)
	job.ErrorCode = cloneStringPtr(update.ErrorCode)
	job.ErrorMessage = cloneStringPtr(update.ErrorMessage)
	if update.Attempts != nil {
		job.Attempts = *update.Attempts
	}
	if update.StartedAt != nil {
		job.StartedAt = cloneTimePtr(update.StartedAt)
	}
	if update.FinishedAt != nil {
		job.FinishedAt = cloneTimePtr(update.FinishedAt)
	}
	job.UpdatedAt = update.UpdatedAt
	r.jobs[id] = job
	return cloneJob(job), nil
}

func (r *MemoryRepository) ClaimProcessingJob(ctx context.Context, id string, update service.JobStateUpdate) (service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.ProcessingJob{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[id]
	if !exists {
		return service.ProcessingJob{}, service.ErrNotFound
	}
	isStaleRunning := job.Status == service.JobStatusRunning &&
		update.StaleRunningBefore != nil &&
		job.UpdatedAt.Before(*update.StaleRunningBefore)
	if job.Status != service.JobStatusQueued && job.Status != service.JobStatusFailed && !isStaleRunning {
		return service.ProcessingJob{}, service.ErrConflict
	}
	if job.MaxAttempts > 0 && job.Attempts >= job.MaxAttempts {
		return service.ProcessingJob{}, service.ErrConflict
	}
	job.Status = update.Status
	job.ProgressPercent = update.ProgressPercent
	job.CurrentStage = cloneStringPtr(update.CurrentStage)
	job.Message = cloneStringPtr(update.Message)
	job.ErrorCode = nil
	job.ErrorMessage = nil
	job.Attempts++
	if update.StartedAt != nil {
		job.StartedAt = cloneTimePtr(update.StartedAt)
	}
	job.FinishedAt = nil
	job.UpdatedAt = update.UpdatedAt
	r.jobs[id] = job
	return cloneJob(job), nil
}

func (r *MemoryRepository) UpdateDocumentProcessingState(ctx context.Context, id string, update service.DocumentStateUpdate) (service.KnowledgeDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeDocument{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, exists := r.documents[id]
	if !exists || doc.DeletedAt != nil {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	doc.Status = update.Status
	doc.ErrorCode = cloneStringPtr(update.ErrorCode)
	doc.ErrorMessage = cloneStringPtr(update.ErrorMessage)
	doc.UpdatedAt = update.UpdatedAt
	r.documents[id] = doc
	return cloneDocument(r.hydrateDocumentLocked(doc)), nil
}

func (r *MemoryRepository) CompleteIngestion(ctx context.Context, input service.CompleteIngestionRecord) (service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.ProcessingJob{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, docExists := r.documents[input.DocumentID]
	job, jobExists := r.jobs[input.JobID]
	if !docExists || !jobExists || doc.DeletedAt != nil {
		return service.ProcessingJob{}, service.ErrNotFound
	}
	if input.ExpectedAttempts != nil && (job.Attempts != *input.ExpectedAttempts || job.Status != service.JobStatusRunning) {
		return service.ProcessingJob{}, service.ErrConflict
	}
	for chunkID, chunk := range r.chunks {
		if chunk.DocumentID == input.DocumentID {
			delete(r.chunks, chunkID)
		}
	}
	for _, chunk := range input.Chunks {
		if _, exists := r.chunks[chunk.ID]; exists {
			return service.ProcessingJob{}, service.ErrConflict
		}
		r.chunks[chunk.ID] = cloneChunk(chunk)
	}
	doc.Status = service.DocumentStatusReady
	doc.ErrorCode = nil
	doc.ErrorMessage = nil
	if input.ParserBackend != nil {
		doc.ParserBackend = cloneStringPtr(input.ParserBackend)
	}
	doc.UpdatedAt = input.UpdatedAt
	r.documents[doc.ID] = doc

	stage := "completed"
	message := "document ingestion completed"
	job.Status = service.JobStatusSucceeded
	job.CurrentStage = &stage
	job.ProgressPercent = 100
	job.Message = &message
	job.ErrorCode = nil
	job.ErrorMessage = nil
	job.FinishedAt = &input.FinishedAt
	job.UpdatedAt = input.UpdatedAt
	r.jobs[job.ID] = job
	return cloneJob(job), nil
}

func (r *MemoryRepository) ListChunks(ctx context.Context, documentID string, scope service.AccessScope, page service.PageInput) (service.ChunkList, error) {
	if err := ctx.Err(); err != nil {
		return service.ChunkList{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, exists := r.documents[documentID]
	if !exists || doc.DeletedAt != nil {
		return service.ChunkList{}, service.ErrNotFound
	}
	kb, exists := r.knowledgeBases[doc.KnowledgeBaseID]
	if !exists || kb.DeletedAt != nil || (!canRead(doc.CreatedBy, scope) && !canRead(kb.CreatedBy, scope)) {
		return service.ChunkList{}, service.ErrNotFound
	}
	items := make([]service.DocumentChunk, 0)
	for _, chunk := range r.chunks {
		if chunk.DocumentID == documentID {
			items = append(items, cloneChunk(chunk))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ChunkIndex == items[j].ChunkIndex {
			return items[i].ID < items[j].ID
		}
		return items[i].ChunkIndex < items[j].ChunkIndex
	})
	total := int64(len(items))
	items = paginate(items, page)
	return service.ChunkList{
		Items: items,
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) ListDocumentsByKnowledgeBase(ctx context.Context, knowledgeBaseID string, status *service.DocumentStatus, scope service.AccessScope, page service.PageInput) (service.DocumentList, error) {
	if err := ctx.Err(); err != nil {
		return service.DocumentList{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	kb, exists := r.knowledgeBases[knowledgeBaseID]
	if !exists || kb.DeletedAt != nil || !canRead(kb.CreatedBy, scope) {
		return service.DocumentList{}, service.ErrNotFound
	}

	items := make([]service.KnowledgeDocument, 0)
	for _, doc := range r.documents {
		if doc.KnowledgeBaseID != knowledgeBaseID || doc.DeletedAt != nil {
			continue
		}
		if status != nil && doc.Status != *status {
			continue
		}
		if !canRead(doc.CreatedBy, scope) && !canRead(kb.CreatedBy, scope) {
			continue
		}
		items = append(items, r.hydrateDocumentLocked(doc))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := int64(len(items))
	items = paginate(items, page)
	return service.DocumentList{
		Items: cloneDocuments(items),
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) GetDocument(ctx context.Context, id string, scope service.AccessScope) (service.KnowledgeDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeDocument{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, exists := r.documents[id]
	if !exists || doc.DeletedAt != nil {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	kb, exists := r.knowledgeBases[doc.KnowledgeBaseID]
	if !exists || kb.DeletedAt != nil {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	if !canRead(doc.CreatedBy, scope) && !canRead(kb.CreatedBy, scope) {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	return cloneDocument(r.hydrateDocumentLocked(doc)), nil
}

func (r *MemoryRepository) SeedKnowledgeBase(kb service.KnowledgeBase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.knowledgeBases[kb.ID] = cloneKnowledgeBase(kb)
}

func (r *MemoryRepository) SeedDocument(doc service.KnowledgeDocument) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.documents[doc.ID] = cloneDocument(doc)
}

func (r *MemoryRepository) hydrateKnowledgeBaseLocked(kb service.KnowledgeBase) service.KnowledgeBase {
	kb.DocumentCount = 0
	kb.ChunkCount = 0
	for _, doc := range r.documents {
		if doc.KnowledgeBaseID == kb.ID && doc.DeletedAt == nil {
			kb.DocumentCount++
			kb.ChunkCount += r.documentChunkCountLocked(doc.ID)
		}
	}
	return cloneKnowledgeBase(kb)
}

func (r *MemoryRepository) hydrateDocumentLocked(doc service.KnowledgeDocument) service.KnowledgeDocument {
	if count := r.documentChunkCountLocked(doc.ID); count > 0 || doc.ChunkCount == 0 {
		doc.ChunkCount = count
	}
	return cloneDocument(doc)
}

func (r *MemoryRepository) documentChunkCountLocked(documentID string) int64 {
	var count int64
	for _, chunk := range r.chunks {
		if chunk.DocumentID == documentID {
			count++
		}
	}
	return count
}

func canRead(createdBy string, scope service.AccessScope) bool {
	return scope.CanReadAll || createdBy == scope.UserID
}

func paginate[T any](items []T, page service.PageInput) []T {
	start := (page.Page - 1) * page.PageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func cloneKnowledgeBases(items []service.KnowledgeBase) []service.KnowledgeBase {
	out := make([]service.KnowledgeBase, len(items))
	for i, item := range items {
		out[i] = cloneKnowledgeBase(item)
	}
	return out
}

func cloneKnowledgeBase(kb service.KnowledgeBase) service.KnowledgeBase {
	kb.ChunkStrategy = cloneRaw(kb.ChunkStrategy)
	kb.RetrievalStrategy = cloneRaw(kb.RetrievalStrategy)
	if kb.DeletedAt != nil {
		value := *kb.DeletedAt
		kb.DeletedAt = &value
	}
	return kb
}

func cloneDocuments(items []service.KnowledgeDocument) []service.KnowledgeDocument {
	out := make([]service.KnowledgeDocument, len(items))
	for i, item := range items {
		out[i] = cloneDocument(item)
	}
	return out
}

func cloneDocument(doc service.KnowledgeDocument) service.KnowledgeDocument {
	doc.Tags = append([]string(nil), doc.Tags...)
	doc.FileRef = cloneStringPtr(doc.FileRef)
	doc.ContentType = cloneStringPtr(doc.ContentType)
	doc.SizeBytes = cloneInt64Ptr(doc.SizeBytes)
	doc.ErrorCode = cloneStringPtr(doc.ErrorCode)
	doc.ErrorMessage = cloneStringPtr(doc.ErrorMessage)
	doc.ParserBackend = cloneStringPtr(doc.ParserBackend)
	doc.CurrentJobID = cloneStringPtr(doc.CurrentJobID)
	if doc.DeletedAt != nil {
		value := *doc.DeletedAt
		doc.DeletedAt = &value
	}
	return doc
}

func cloneJob(job service.ProcessingJob) service.ProcessingJob {
	job.DocumentID = cloneStringPtr(job.DocumentID)
	job.CurrentStage = cloneStringPtr(job.CurrentStage)
	job.Message = cloneStringPtr(job.Message)
	job.ErrorCode = cloneStringPtr(job.ErrorCode)
	job.ErrorMessage = cloneStringPtr(job.ErrorMessage)
	job.ParserConfigID = cloneStringPtr(job.ParserConfigID)
	job.ParserConfigSnapshot = cloneRaw(job.ParserConfigSnapshot)
	if job.StartedAt != nil {
		value := *job.StartedAt
		job.StartedAt = &value
	}
	if job.FinishedAt != nil {
		value := *job.FinishedAt
		job.FinishedAt = &value
	}
	return job
}

func cloneChunk(chunk service.DocumentChunk) service.DocumentChunk {
	chunk.SectionPath = cloneStringPtr(chunk.SectionPath)
	chunk.TokenCount = cloneInt32Ptr(chunk.TokenCount)
	chunk.ChunkType = cloneStringPtr(chunk.ChunkType)
	chunk.QdrantPointID = cloneStringPtr(chunk.QdrantPointID)
	chunk.EmbeddingProvider = cloneStringPtr(chunk.EmbeddingProvider)
	chunk.EmbeddingModel = cloneStringPtr(chunk.EmbeddingModel)
	chunk.EmbeddingDimension = cloneInt32Ptr(chunk.EmbeddingDimension)
	if chunk.Metadata == nil {
		chunk.Metadata = map[string]any{}
	} else {
		metadata := make(map[string]any, len(chunk.Metadata))
		for key, value := range chunk.Metadata {
			metadata[key] = value
		}
		chunk.Metadata = metadata
	}
	return chunk
}

func cloneRaw(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value...)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt32Ptr(value *int32) *int32 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
