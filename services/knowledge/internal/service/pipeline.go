package service

import (
	"context"
	"io"
	"strings"
	"time"
)

type SourceDocument struct {
	Body        io.ReadCloser
	ContentType string
	SizeBytes   int64
}

type SourceReader interface {
	ReadSource(ctx context.Context, fileID string) (SourceDocument, error)
}

type ParseInput struct {
	Name        string
	ContentType string
	Body        io.Reader
	SizeBytes   int64
}

type ParsedDocument struct {
	Content string
	Title   string
}

type Parser interface {
	Parse(ctx context.Context, input ParseInput) (ParsedDocument, error)
}

type ChunkInput struct {
	Content  string
	Strategy ChunkStrategy
}

type ChunkSpec struct {
	SectionPath *string
	Content     string
	TokenCount  int
	ChunkType   *string
	Metadata    map[string]any
}

type Chunker interface {
	Chunk(ctx context.Context, input ChunkInput) ([]ChunkSpec, error)
}

type DocumentStateUpdate struct {
	Status        DocumentStatus
	ErrorCode     *string
	ErrorMessage  *string
	ParsedContent *string
	ChunkCount    *int
	UpdatedAt     time.Time
}

type JobStateUpdate struct {
	Status          JobStatus
	CurrentStage    *JobStage
	ProgressPercent int
	Message         *string
	ErrorCode       *string
	ErrorMessage    *string
	Attempts        *int
	StartedAt       *time.Time
	FinishedAt      *time.Time
	UpdatedAt       time.Time
}

func (s *KnowledgeService) ProcessIngestionJob(ctx context.Context, reqCtx RequestContext, jobID string) (ProcessingJob, error) {
	job, err := s.GetJob(ctx, reqCtx, jobID)
	if err != nil {
		return ProcessingJob{}, err
	}
	if job.JobType != JobTypeIngest {
		return ProcessingJob{}, ConflictError("job type is not supported by ingestion pipeline", nil)
	}
	if job.DocumentID == nil {
		return ProcessingJob{}, ConflictError("job has no document", nil)
	}
	if job.Status != JobStatusQueued && job.Status != JobStatusFailed {
		return ProcessingJob{}, ConflictError("job is not ready to run", nil)
	}
	if s.sourceReader == nil || s.parser == nil || s.chunker == nil {
		failed := s.failProcessing(ctx, job, *job.DocumentID, "dependency_error", "processing pipeline is not configured")
		return failed, DependencyError("processing pipeline is not configured", nil)
	}

	doc, err := s.repo.FindDocumentByID(ctx, *job.DocumentID)
	if err != nil {
		return ProcessingJob{}, mapDocumentRepositoryError(err, "document not found", "document metadata access failed")
	}
	base, err := s.repo.FindKnowledgeBaseByID(ctx, doc.KnowledgeBaseID)
	if err != nil {
		return ProcessingJob{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}

	startedAt := s.now().UTC()
	attempts := job.Attempts + 1
	parsingStage := JobStage("parsing")
	if _, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
		Status:          JobStatusRunning,
		CurrentStage:    &parsingStage,
		ProgressPercent: 20,
		Attempts:        &attempts,
		StartedAt:       &startedAt,
		UpdatedAt:       startedAt,
	}); err != nil {
		return ProcessingJob{}, DependencyError("job state update failed", err)
	}
	if _, err := s.repo.UpdateDocumentProcessingState(ctx, doc.ID, DocumentStateUpdate{
		Status:    DocumentStatusParsing,
		UpdatedAt: startedAt,
	}); err != nil {
		return ProcessingJob{}, DependencyError("document state update failed", err)
	}

	source, err := s.sourceReader.ReadSource(ctx, doc.FileID)
	if err != nil {
		failed := s.failProcessing(ctx, job, doc.ID, "dependency_error", "source content read failed")
		return failed, DependencyError("source content read failed", err)
	}
	defer source.Body.Close()

	contentType := source.ContentType
	if contentType == "" && doc.ContentType != nil {
		contentType = *doc.ContentType
	}
	parsed, err := s.parser.Parse(ctx, ParseInput{
		Name:        doc.Name,
		ContentType: contentType,
		Body:        source.Body,
		SizeBytes:   source.SizeBytes,
	})
	if err != nil {
		failed := s.failProcessing(ctx, job, doc.ID, "parse_failed", "document parsing failed")
		return failed, ValidationError("document parsing failed", map[string]string{"file": "could not be parsed"})
	}

	chunkingAt := s.now().UTC()
	chunkingStage := JobStage("chunking")
	if _, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
		Status:          JobStatusRunning,
		CurrentStage:    &chunkingStage,
		ProgressPercent: 60,
		UpdatedAt:       chunkingAt,
	}); err != nil {
		return ProcessingJob{}, DependencyError("job state update failed", err)
	}
	if _, err := s.repo.UpdateDocumentProcessingState(ctx, doc.ID, DocumentStateUpdate{
		Status:        DocumentStatusChunking,
		ParsedContent: &parsed.Content,
		UpdatedAt:     chunkingAt,
	}); err != nil {
		return ProcessingJob{}, DependencyError("document state update failed", err)
	}

	chunkSpecs, err := s.chunker.Chunk(ctx, ChunkInput{
		Content:  parsed.Content,
		Strategy: base.ChunkStrategy,
	})
	if err != nil {
		failed := s.failProcessing(ctx, job, doc.ID, "chunk_failed", "document chunking failed")
		return failed, ValidationError("document chunking failed", map[string]string{"content": "could not be chunked"})
	}
	chunks := make([]DocumentChunk, 0, len(chunkSpecs))
	for index, spec := range chunkSpecs {
		chunkID, err := s.newID("chunk")
		if err != nil {
			failed := s.failProcessing(ctx, job, doc.ID, "dependency_error", "chunk id generation failed")
			return failed, DependencyError("chunk id generation failed", err)
		}
		chunks = append(chunks, DocumentChunk{
			ID:              chunkID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			DocumentID:      doc.ID,
			ChunkIndex:      index,
			SectionPath:     spec.SectionPath,
			Content:         spec.Content,
			TokenCount:      spec.TokenCount,
			ChunkType:       spec.ChunkType,
			Metadata:        spec.Metadata,
			CreatedAt:       s.now().UTC(),
		})
	}
	vectorsIndexed := false
	if s.embedder != nil && s.vectorIndex != nil {
		embeddingAt := s.now().UTC()
		embeddingStage := JobStage("embedding")
		if _, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
			Status:          JobStatusRunning,
			CurrentStage:    &embeddingStage,
			ProgressPercent: 80,
			UpdatedAt:       embeddingAt,
		}); err != nil {
			return ProcessingJob{}, DependencyError("job state update failed", err)
		}
		if _, err := s.repo.UpdateDocumentProcessingState(ctx, doc.ID, DocumentStateUpdate{
			Status:    DocumentStatusEmbedding,
			UpdatedAt: embeddingAt,
		}); err != nil {
			return ProcessingJob{}, DependencyError("document state update failed", err)
		}
		texts := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			texts = append(texts, chunk.Content)
		}
		embeddings, err := s.embedder.Embed(ctx, EmbeddingRequest{Texts: texts})
		if err != nil {
			failed := s.failProcessing(ctx, job, doc.ID, "embedding_failed", "document embedding failed")
			return failed, DependencyError("document embedding failed", err)
		}
		if len(embeddings.Vectors) != len(chunks) {
			failed := s.failProcessing(ctx, job, doc.ID, "embedding_failed", "embedding result count mismatch")
			return failed, DependencyError("document embedding failed", nil)
		}
		if err := s.vectorIndex.DeleteByDocument(ctx, doc.ID); err != nil {
			failed := s.failProcessing(ctx, job, doc.ID, "index_failed", "vector cleanup failed")
			return failed, DependencyError("vector cleanup failed", err)
		}
		points := make([]VectorPoint, 0, len(chunks))
		for index := range chunks {
			pointID := stableVectorPointID(chunks[index].ID)
			chunks[index].QdrantPointID = &pointID
			chunks[index].EmbeddingProvider = &embeddings.Provider
			chunks[index].EmbeddingModel = &embeddings.Model
			chunks[index].EmbeddingDimension = &embeddings.Dimension
			points = append(points, VectorPoint{
				ID:     pointID,
				Vector: embeddings.Vectors[index],
				Payload: map[string]any{
					"knowledge_base_id": chunks[index].KnowledgeBaseID,
					"document_id":       chunks[index].DocumentID,
					"chunk_id":          chunks[index].ID,
					"chunk_index":       chunks[index].ChunkIndex,
					"chunk_type":        derefString(chunks[index].ChunkType),
					"section_path":      derefString(chunks[index].SectionPath),
					"tags":              append([]string(nil), doc.Tags...),
					"metadata":          chunks[index].Metadata,
				},
			})
		}
		indexingAt := s.now().UTC()
		indexingStage := JobStage("indexing")
		if _, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
			Status:          JobStatusRunning,
			CurrentStage:    &indexingStage,
			ProgressPercent: 90,
			UpdatedAt:       indexingAt,
		}); err != nil {
			return ProcessingJob{}, DependencyError("job state update failed", err)
		}
		if err := s.vectorIndex.Upsert(ctx, points); err != nil {
			failed := s.failProcessing(ctx, job, doc.ID, "index_failed", "vector indexing failed")
			return failed, DependencyError("vector indexing failed", err)
		}
		vectorsIndexed = true
	}
	if err := s.repo.ReplaceDocumentChunks(ctx, doc.ID, chunks); err != nil {
		if vectorsIndexed && s.vectorIndex != nil {
			_ = s.vectorIndex.DeleteByDocument(ctx, doc.ID)
		}
		failed := s.failProcessing(ctx, job, doc.ID, "dependency_error", "document chunks write failed")
		return failed, DependencyError("document chunks write failed", err)
	}

	finishedAt := s.now().UTC()
	chunkCount := len(chunks)
	if _, err := s.repo.UpdateDocumentProcessingState(ctx, doc.ID, DocumentStateUpdate{
		Status:     DocumentStatusReady,
		ChunkCount: &chunkCount,
		UpdatedAt:  finishedAt,
	}); err != nil {
		return ProcessingJob{}, DependencyError("document state update failed", err)
	}
	doneStage := JobStage("done")
	finished, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
		Status:          JobStatusSucceeded,
		CurrentStage:    &doneStage,
		ProgressPercent: 100,
		FinishedAt:      &finishedAt,
		UpdatedAt:       finishedAt,
	})
	if err != nil {
		return ProcessingJob{}, DependencyError("job state update failed", err)
	}
	return finished, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *KnowledgeService) failProcessing(ctx context.Context, job ProcessingJob, documentID string, errorCode string, message string) ProcessingJob {
	now := s.now().UTC()
	code := strings.TrimSpace(errorCode)
	stage := JobStage("failed")
	_, _ = s.repo.UpdateDocumentProcessingState(ctx, documentID, DocumentStateUpdate{
		Status:       DocumentStatusFailed,
		ErrorCode:    &code,
		ErrorMessage: &message,
		UpdatedAt:    now,
	})
	failed, err := s.repo.UpdateJobState(ctx, job.ID, JobStateUpdate{
		Status:          JobStatusFailed,
		CurrentStage:    &stage,
		ProgressPercent: job.ProgressPercent,
		ErrorCode:       &code,
		ErrorMessage:    &message,
		FinishedAt:      &now,
		UpdatedAt:       now,
	})
	if err != nil {
		job.Status = JobStatusFailed
		job.CurrentStage = &stage
		job.ErrorCode = &code
		job.ErrorMessage = &message
		job.FinishedAt = &now
		job.UpdatedAt = now
		return job
	}
	return failed
}
