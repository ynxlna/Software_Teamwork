package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/hibiken/asynq"
)

func TestWorkerRecordsOperationLogsForJobStatusTransitions(t *testing.T) {
	payload := ReportJobPayload{
		RequestID: "req-worker",
		JobType:   string(service.JobTypeContentGeneration),
		JobID:     "job-1",
		AttemptID: "attempt-1",
		UserID:    "user-1",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	manager := &fakeWorkerJobManager{}
	worker := &Worker{
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		jobsMgr: manager,
	}

	if err := worker.handleReportJob(context.Background(), asynq.NewTask(TaskContentGeneration, raw)); err != nil {
		t.Fatalf("handleReportJob() error = %v", err)
	}

	if len(manager.logs) != 2 {
		t.Fatalf("operation log count = %d, want 2", len(manager.logs))
	}
	if got := manager.logs[0]; got.OperationType != service.OperationReportJobRunning || got.TargetID != "job-1" || got.RequestSource != "worker" {
		t.Fatalf("running operation log = %+v", got)
	}
	if got := manager.logs[1]; got.OperationType != service.OperationReportJobSucceeded || got.TargetID != "job-1" || got.ParameterSummary["jobType"] != string(service.JobTypeContentGeneration) {
		t.Fatalf("succeeded operation log = %+v", got)
	}
}

func TestWorkerSanitizesOperationLogSummaries(t *testing.T) {
	payload := ReportJobPayload{
		RequestID: "req-worker",
		JobType:   "content_generation prompt=secret https://minio.local/bucket/object",
		JobID:     "job-1",
		AttemptID: "attempt-1",
		UserID:    "user-1",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	manager := &fakeWorkerJobManager{}
	worker := &Worker{
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		jobsMgr: manager,
	}

	if err := worker.handleReportJob(context.Background(), asynq.NewTask(TaskContentGeneration, raw)); err != nil {
		t.Fatalf("handleReportJob() error = %v", err)
	}

	if len(manager.logs) != 2 {
		t.Fatalf("operation log count = %d, want 2", len(manager.logs))
	}
	for _, log := range manager.logs {
		if got := log.ParameterSummary["jobType"]; got != "[redacted]" {
			t.Fatalf("operation log jobType summary was not sanitized: %+v", log.ParameterSummary)
		}
	}
}

func TestWorkerExecutesReportFileCreationJob(t *testing.T) {
	mgr := &fakeWorkerJobManager{}
	executor := &fakeReportFileExecutor{}
	w := &Worker{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		jobsMgr:            mgr,
		reportFileExecutor: executor,
	}
	payload := ReportJobPayload{
		RequestID: "req-1",
		JobType:   string(service.JobTypeReportFileCreation),
		JobID:     "job-1",
		AttemptID: "attempt-1",
		UserID:    "user-1",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := w.handleReportJob(context.Background(), asynq.NewTask(TaskReportFileCreation, body)); err != nil {
		t.Fatalf("handleReportJob() error = %v", err)
	}
	if executor.payload.JobID != "job-1" || executor.payload.UserID != "user-1" {
		t.Fatalf("executor payload = %+v", executor.payload)
	}
	if !mgr.jobRunning || !mgr.attemptRunning || !mgr.jobSucceeded || !mgr.attemptSucceeded {
		t.Fatalf("expected running and succeeded state transitions, got %+v", mgr)
	}
}

func TestWorkerRecordsFailedOperationLogWhenJobUpdateFails(t *testing.T) {
	payload := ReportJobPayload{
		RequestID: "req-worker-failed",
		JobType:   string(service.JobTypeContentGeneration),
		JobID:     "job-1",
		AttemptID: "attempt-1",
		UserID:    "user-1",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	manager := &fakeWorkerJobManager{succeededErr: errors.New("postgres unavailable")}
	worker := &Worker{
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		jobsMgr: manager,
	}

	if err := worker.handleReportJob(context.Background(), asynq.NewTask(TaskContentGeneration, raw)); err == nil {
		t.Fatal("handleReportJob() error = nil, want state update error")
	}

	if len(manager.logs) != 2 {
		t.Fatalf("operation log count = %d, want running and failed", len(manager.logs))
	}
	if got := manager.logs[1]; got.OperationType != service.OperationReportJobFailed || got.OperationResult != service.OperationResultFailed || got.TargetID != "job-1" {
		t.Fatalf("failed operation log = %+v", got)
	}
}

func TestWorkerRecordsFailedOperationLogWhenReportFileExecutionFails(t *testing.T) {
	payload := ReportJobPayload{
		RequestID: "req-file-failed",
		JobType:   string(service.JobTypeReportFileCreation),
		JobID:     "job-1",
		AttemptID: "attempt-1",
		UserID:    "user-1",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	manager := &fakeWorkerJobManager{}
	executor := &fakeReportFileExecutor{err: errors.New("file service unavailable")}
	worker := &Worker{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		jobsMgr:            manager,
		reportFileExecutor: executor,
	}

	if err := worker.handleReportJob(context.Background(), asynq.NewTask(TaskReportFileCreation, raw)); err == nil {
		t.Fatal("handleReportJob() error = nil, want execution error")
	}

	if len(manager.logs) != 2 {
		t.Fatalf("operation log count = %d, want running and failed", len(manager.logs))
	}
	if !manager.jobFailed || !manager.attemptFailed {
		t.Fatalf("expected failed state transitions, got %+v", manager)
	}
	if got := manager.logs[1]; got.OperationType != service.OperationReportJobFailed || got.OperationResult != service.OperationResultFailed || got.TargetID != "job-1" {
		t.Fatalf("failed operation log = %+v", got)
	}
}

type fakeWorkerJobManager struct {
	jobRunning       bool
	jobSucceeded     bool
	jobFailed        bool
	attemptRunning   bool
	attemptSucceeded bool
	attemptFailed    bool
	logs             []service.OperationLog
	succeededErr     error
}

func (f *fakeWorkerJobManager) SetJobRunning(context.Context, string) error {
	f.jobRunning = true
	return nil
}

func (f *fakeWorkerJobManager) SetJobSucceeded(context.Context, string) error {
	if f.succeededErr != nil {
		return f.succeededErr
	}
	f.jobSucceeded = true
	return nil
}

func (f *fakeWorkerJobManager) SetJobFailed(context.Context, string, string, string) error {
	f.jobFailed = true
	return nil
}

func (f *fakeWorkerJobManager) SetAttemptRunning(context.Context, string) error {
	f.attemptRunning = true
	return nil
}

func (f *fakeWorkerJobManager) SetAttemptSucceeded(context.Context, string) error {
	f.attemptSucceeded = true
	return nil
}

func (f *fakeWorkerJobManager) SetAttemptFailed(context.Context, string, string, string) error {
	f.attemptFailed = true
	return nil
}

func (f *fakeWorkerJobManager) CreateOperationLog(_ context.Context, log service.OperationLog) (service.OperationLog, error) {
	f.logs = append(f.logs, log)
	return log, nil
}

type fakeReportFileExecutor struct {
	payload service.ReportFileExecutionPayload
	err     error
}

func (f *fakeReportFileExecutor) ExecuteReportFileCreation(_ context.Context, payload service.ReportFileExecutionPayload) error {
	f.payload = payload
	return f.err
}
