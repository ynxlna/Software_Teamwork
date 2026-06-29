package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

type JobRepository interface {
	FindReportJobByID(ctx context.Context, id string) (ReportJob, error)
	ListReportJobsByReportID(ctx context.Context, reportID string) ([]ReportJob, error)
	UpdateReportJobStatus(ctx context.Context, id string, status JobStatus, errorCode, errorMessage string, startedAt, finishedAt *time.Time) (ReportJob, error)
	IncrementJobRetryCount(ctx context.Context, id string) (ReportJob, error)
	CreateReportJobAttempt(ctx context.Context, value ReportJobAttempt) (ReportJobAttempt, error)
	ListReportJobAttemptsByJobID(ctx context.Context, jobID string) ([]ReportJobAttempt, error)
	ListReportEventsByReportID(ctx context.Context, reportID string) ([]ReportEvent, error)
}

type JobService struct {
	repo JobRepository
}

func NewJobService(repo JobRepository) *JobService {
	return &JobService{repo: repo}
}

func (s *JobService) GetJob(ctx context.Context, id string) (ReportJob, error) {
	return s.repo.FindReportJobByID(ctx, id)
}

func (s *JobService) ListJobs(ctx context.Context, reportID string) ([]ReportJob, error) {
	return s.repo.ListReportJobsByReportID(ctx, reportID)
}

func (s *JobService) CancelJob(ctx context.Context, id string) (ReportJob, error) {
	job, err := s.repo.FindReportJobByID(ctx, id)
	if err != nil {
		return ReportJob{}, err
	}
	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return ReportJob{}, NewError(CodeValidation, "job cannot be canceled in current status", nil)
	}
	return s.repo.UpdateReportJobStatus(ctx, id, JobStatusCanceled, "", "", nil, nil)
}

func (s *JobService) RetryJob(ctx context.Context, id string) (ReportJobAttempt, error) {
	job, err := s.repo.FindReportJobByID(ctx, id)
	if err != nil {
		return ReportJobAttempt{}, err
	}
	if job.RetryCount >= job.MaxAttempts {
		return ReportJobAttempt{}, NewError(CodeValidation, "max retry attempts reached", nil)
	}
	attempt := ReportJobAttempt{
		ID:            newID(),
		JobID:         job.ID,
		AttemptNumber: job.RetryCount + 1,
		TriggerSource: "user",
		Status:        JobStatusPending,
		CreatedAt:     time.Now().UTC(),
	}
	attempt, err = s.repo.CreateReportJobAttempt(ctx, attempt)
	if err != nil {
		return ReportJobAttempt{}, fmt.Errorf("create retry attempt: %w", err)
	}
	if _, err = s.repo.IncrementJobRetryCount(ctx, id); err != nil {
		return ReportJobAttempt{}, fmt.Errorf("increment retry count: %w", err)
	}
	return attempt, nil
}

func (s *JobService) ListAttempts(ctx context.Context, jobID string) ([]ReportJobAttempt, error) {
	return s.repo.ListReportJobAttemptsByJobID(ctx, jobID)
}

func (s *JobService) ListEvents(ctx context.Context, reportID string) ([]ReportEvent, error) {
	return s.repo.ListReportEventsByReportID(ctx, reportID)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
