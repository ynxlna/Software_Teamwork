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
	chunks         map[string][]service.DocumentChunk
	jobs           map[string]service.ProcessingJob
	jobsByKey      map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		knowledgeBases: map[string]service.KnowledgeBase{},
		documents:      map[string]service.KnowledgeDocument{},
		chunks:         map[string][]service.DocumentChunk{},
		jobs:           map[string]service.ProcessingJob{},
		jobsByKey:      map[string]string{},
	}
}

func (r *MemoryRepository) CreateKnowledgeBase(ctx context.Context, base service.KnowledgeBase) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.knowledgeBases[base.ID]; exists {
		return service.KnowledgeBase{}, service.ErrConflict
	}
	stored := cloneKnowledgeBase(base)
	r.knowledgeBases[stored.ID] = stored
	return cloneKnowledgeBase(stored), nil
}

func (r *MemoryRepository) ListKnowledgeBases(ctx context.Context, filter service.KnowledgeBaseFilter) (service.KnowledgeBaseList, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBaseList{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	docType := strings.TrimSpace(filter.DocType)
	owner := strings.TrimSpace(filter.OwnerUserID)
	items := make([]service.KnowledgeBase, 0, len(r.knowledgeBases))
	for _, base := range r.knowledgeBases {
		if base.DeletedAt != nil {
			continue
		}
		if owner != "" && base.CreatedBy != owner {
			continue
		}
		if docType != "" && base.DocType != docType {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(base.Name), keyword) && !strings.Contains(strings.ToLower(base.Description), keyword) {
			continue
		}
		items = append(items, cloneKnowledgeBase(base))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = nil
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	return service.KnowledgeBaseList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) FindKnowledgeBaseByID(ctx context.Context, id string) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	base, exists := r.knowledgeBases[id]
	if !exists || base.DeletedAt != nil {
		return service.KnowledgeBase{}, service.ErrNotFound
	}
	return cloneKnowledgeBase(base), nil
}

func (r *MemoryRepository) UpdateKnowledgeBase(ctx context.Context, base service.KnowledgeBase) (service.KnowledgeBase, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeBase{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.knowledgeBases[base.ID]
	if !exists || existing.DeletedAt != nil {
		return service.KnowledgeBase{}, service.ErrNotFound
	}
	stored := cloneKnowledgeBase(base)
	r.knowledgeBases[stored.ID] = stored
	return cloneKnowledgeBase(stored), nil
}

func (r *MemoryRepository) MarkKnowledgeBaseDeleted(ctx context.Context, id string, deletedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	base, exists := r.knowledgeBases[id]
	if !exists || base.DeletedAt != nil {
		return service.ErrNotFound
	}
	deleted := deletedAt.UTC()
	base.DeletedAt = &deleted
	base.UpdatedAt = deleted
	r.knowledgeBases[id] = base
	return nil
}

func (r *MemoryRepository) PutDocumentForTest(doc service.KnowledgeDocument) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.documents[doc.ID] = cloneDocument(doc)
}

func (r *MemoryRepository) PutChunksForTest(documentID string, chunks []service.DocumentChunk) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]service.DocumentChunk, 0, len(chunks))
	for _, chunk := range chunks {
		copied = append(copied, cloneChunk(chunk))
	}
	r.chunks[documentID] = copied
}

func (r *MemoryRepository) CreateIngestionJob(ctx context.Context, doc service.KnowledgeDocument, job service.ProcessingJob) (service.KnowledgeDocument, service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if job.IdempotencyKey != nil {
		if existingJobID, ok := r.jobsByKey[*job.IdempotencyKey]; ok {
			existingJob, exists := r.jobs[existingJobID]
			if !exists {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
			}
			if existingJob.DocumentID == nil {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
			}
			existingDoc, exists := r.documents[*existingJob.DocumentID]
			if !exists {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
			}
			return cloneDocument(existingDoc), cloneJob(existingJob), nil
		}
	}
	if _, exists := r.documents[doc.ID]; exists {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
	}
	if _, exists := r.jobs[job.ID]; exists {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
	}
	storedDoc := cloneDocument(doc)
	storedJob := cloneJob(job)
	r.documents[storedDoc.ID] = storedDoc
	r.jobs[storedJob.ID] = storedJob
	if storedJob.IdempotencyKey != nil {
		r.jobsByKey[*storedJob.IdempotencyKey] = storedJob.ID
	}
	return cloneDocument(storedDoc), cloneJob(storedJob), nil
}

func (r *MemoryRepository) FindJobByID(ctx context.Context, id string) (service.ProcessingJob, error) {
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

func (r *MemoryRepository) CreateProcessingJob(ctx context.Context, job service.ProcessingJob) (service.ProcessingJob, error) {
	if err := ctx.Err(); err != nil {
		return service.ProcessingJob{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[job.ID]; exists {
		return service.ProcessingJob{}, service.ErrConflict
	}
	stored := cloneJob(job)
	r.jobs[stored.ID] = stored
	return cloneJob(stored), nil
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
	job.Status = update.Status
	job.CurrentStage = cloneJobStagePtr(update.CurrentStage)
	job.ProgressPercent = update.ProgressPercent
	job.Message = cloneStringPtr(update.Message)
	job.ErrorCode = cloneStringPtr(update.ErrorCode)
	job.ErrorMessage = cloneStringPtr(update.ErrorMessage)
	if update.Attempts != nil {
		job.Attempts = *update.Attempts
	}
	job.StartedAt = cloneTimePtr(update.StartedAt)
	if update.FinishedAt != nil {
		job.FinishedAt = cloneTimePtr(update.FinishedAt)
	}
	job.UpdatedAt = update.UpdatedAt
	r.jobs[id] = job
	return cloneJob(job), nil
}

func (r *MemoryRepository) ListDocuments(ctx context.Context, filter service.DocumentFilter) (service.DocumentList, error) {
	if err := ctx.Err(); err != nil {
		return service.DocumentList{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]service.KnowledgeDocument, 0, len(r.documents))
	for _, doc := range r.documents {
		if doc.DeletedAt != nil {
			continue
		}
		if doc.KnowledgeBaseID != filter.KnowledgeBaseID {
			continue
		}
		if filter.OwnerUserID != "" && doc.CreatedBy != filter.OwnerUserID {
			continue
		}
		if filter.Status != "" && doc.Status != filter.Status {
			continue
		}
		items = append(items, cloneDocument(doc))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = nil
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	return service.DocumentList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) FindDocumentByID(ctx context.Context, id string) (service.KnowledgeDocument, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeDocument{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	doc, exists := r.documents[id]
	if !exists || doc.DeletedAt != nil {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	return cloneDocument(doc), nil
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
	if update.ParsedContent != nil {
		doc.ParsedContent = cloneStringPtr(update.ParsedContent)
	}
	if update.ChunkCount != nil {
		doc.ChunkCount = *update.ChunkCount
	}
	doc.UpdatedAt = &update.UpdatedAt
	r.documents[id] = doc
	return cloneDocument(doc), nil
}

func (r *MemoryRepository) ListChunks(ctx context.Context, filter service.ChunkFilter) (service.ChunkList, error) {
	if err := ctx.Err(); err != nil {
		return service.ChunkList{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	all := r.chunks[filter.DocumentID]
	items := make([]service.DocumentChunk, 0, len(all))
	for _, chunk := range all {
		items = append(items, cloneChunk(chunk))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ChunkIndex == items[j].ChunkIndex {
			return items[i].ID < items[j].ID
		}
		return items[i].ChunkIndex < items[j].ChunkIndex
	})

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = nil
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	return service.ChunkList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *MemoryRepository) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []service.DocumentChunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.documents[documentID]; !exists {
		return service.ErrNotFound
	}
	copied := make([]service.DocumentChunk, 0, len(chunks))
	for _, chunk := range chunks {
		copied = append(copied, cloneChunk(chunk))
	}
	r.chunks[documentID] = copied
	return nil
}

func (r *MemoryRepository) FindChunksByIDs(ctx context.Context, ids []string) ([]service.DocumentChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	wanted := make(map[string]int, len(ids))
	for index, id := range ids {
		if _, exists := wanted[id]; !exists {
			wanted[id] = index
		}
	}
	found := make([]service.DocumentChunk, 0, len(wanted))
	for _, chunks := range r.chunks {
		for _, chunk := range chunks {
			if _, ok := wanted[chunk.ID]; ok {
				found = append(found, cloneChunk(chunk))
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		return wanted[found[i].ID] < wanted[found[j].ID]
	})
	return found, nil
}

func (r *MemoryRepository) GetKnowledgeStats(ctx context.Context, filter service.StatsFilter) (service.KnowledgeStats, error) {
	if err := ctx.Err(); err != nil {
		return service.KnowledgeStats{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	owner := strings.TrimSpace(filter.OwnerUserID)
	stats := service.KnowledgeStats{}
	uploads := map[string]int{}
	visibleBases := map[string]struct{}{}

	for _, base := range r.knowledgeBases {
		if base.DeletedAt != nil {
			continue
		}
		if owner != "" && strings.TrimSpace(base.CreatedBy) != owner {
			continue
		}
		stats.KnowledgeBaseCount++
		visibleBases[base.ID] = struct{}{}
	}

	for _, doc := range r.documents {
		if doc.DeletedAt != nil {
			continue
		}
		if _, ok := visibleBases[doc.KnowledgeBaseID]; !ok {
			continue
		}
		if owner != "" && strings.TrimSpace(doc.CreatedBy) != owner {
			continue
		}
		stats.DocumentCount++
		switch doc.Status {
		case service.DocumentStatusReady:
			stats.ReadyDocumentCount++
		case service.DocumentStatusFailed:
			stats.FailedDocumentCount++
		}
		if !doc.CreatedAt.Before(filter.Since) && (filter.Until.IsZero() || !doc.CreatedAt.After(filter.Until)) {
			uploads[doc.CreatedAt.UTC().Format("2006-01-02")]++
		}
	}

	for documentID, chunks := range r.chunks {
		doc, ok := r.documents[documentID]
		if !ok || doc.DeletedAt != nil {
			continue
		}
		if _, ok := visibleBases[doc.KnowledgeBaseID]; !ok {
			continue
		}
		if owner != "" && strings.TrimSpace(doc.CreatedBy) != owner {
			continue
		}
		stats.ChunkCount += len(chunks)
	}

	stats.RecentUploads = uploadStatsFromMap(uploads)
	return stats, nil
}

func uploadStatsFromMap(input map[string]int) []service.DailyUploadStat {
	items := make([]service.DailyUploadStat, 0, len(input))
	for date, count := range input {
		items = append(items, service.DailyUploadStat{Date: date, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date < items[j].Date
	})
	return items
}

func cloneKnowledgeBase(base service.KnowledgeBase) service.KnowledgeBase {
	base.ChunkStrategy = cloneMap(base.ChunkStrategy)
	base.RetrievalStrategy = cloneMap(base.RetrievalStrategy)
	if base.DeletedAt != nil {
		deletedAt := *base.DeletedAt
		base.DeletedAt = &deletedAt
	}
	return base
}

func cloneDocument(doc service.KnowledgeDocument) service.KnowledgeDocument {
	doc.Tags = append([]string(nil), doc.Tags...)
	if doc.ContentType != nil {
		value := *doc.ContentType
		doc.ContentType = &value
	}
	if doc.ErrorCode != nil {
		value := *doc.ErrorCode
		doc.ErrorCode = &value
	}
	if doc.ErrorMessage != nil {
		value := *doc.ErrorMessage
		doc.ErrorMessage = &value
	}
	if doc.ParsedContent != nil {
		value := *doc.ParsedContent
		doc.ParsedContent = &value
	}
	if doc.ParserBackend != nil {
		value := *doc.ParserBackend
		doc.ParserBackend = &value
	}
	if doc.UpdatedAt != nil {
		value := *doc.UpdatedAt
		doc.UpdatedAt = &value
	}
	if doc.DeletedAt != nil {
		value := *doc.DeletedAt
		doc.DeletedAt = &value
	}
	if doc.CurrentJobID != nil {
		value := *doc.CurrentJobID
		doc.CurrentJobID = &value
	}
	return doc
}

func cloneChunk(chunk service.DocumentChunk) service.DocumentChunk {
	if chunk.SectionPath != nil {
		value := *chunk.SectionPath
		chunk.SectionPath = &value
	}
	if chunk.ChunkType != nil {
		value := *chunk.ChunkType
		chunk.ChunkType = &value
	}
	if chunk.QdrantPointID != nil {
		value := *chunk.QdrantPointID
		chunk.QdrantPointID = &value
	}
	if chunk.EmbeddingProvider != nil {
		value := *chunk.EmbeddingProvider
		chunk.EmbeddingProvider = &value
	}
	if chunk.EmbeddingModel != nil {
		value := *chunk.EmbeddingModel
		chunk.EmbeddingModel = &value
	}
	if chunk.EmbeddingDimension != nil {
		value := *chunk.EmbeddingDimension
		chunk.EmbeddingDimension = &value
	}
	chunk.Metadata = cloneMap(chunk.Metadata)
	return chunk
}

func cloneJob(job service.ProcessingJob) service.ProcessingJob {
	if job.DocumentID != nil {
		value := *job.DocumentID
		job.DocumentID = &value
	}
	if job.CurrentStage != nil {
		value := *job.CurrentStage
		job.CurrentStage = &value
	}
	if job.Message != nil {
		value := *job.Message
		job.Message = &value
	}
	if job.ErrorCode != nil {
		value := *job.ErrorCode
		job.ErrorCode = &value
	}
	if job.ErrorMessage != nil {
		value := *job.ErrorMessage
		job.ErrorMessage = &value
	}
	if job.IdempotencyKey != nil {
		value := *job.IdempotencyKey
		job.IdempotencyKey = &value
	}
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

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneJobStagePtr(value *service.JobStage) *service.JobStage {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneMap[M ~map[string]any](source M) M {
	if source == nil {
		return nil
	}
	clone := make(M, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
