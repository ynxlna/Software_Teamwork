package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/hibiken/asynq"
)

const (
	TaskOutlineGeneration   = "document:report:outline_generation"
	TaskOutlineRegeneration = "document:report:outline_regeneration"
	TaskContentGeneration   = "document:report:content_generation"
	TaskContentRegeneration = "document:report:content_regeneration"
	TaskSectionRegeneration = "document:report:section_regeneration"
	TaskReportFileCreation  = "document:report:report_file_creation"
)

type ReportJobPayload struct {
	RequestID string `json:"requestId"`
	JobType   string `json:"jobType"`
	JobID     string `json:"jobId"`
	AttemptID string `json:"attemptId"`
	UserID    string `json:"userId"`
}

func TaskTypeForJobType(jobType service.JobType) (string, error) {
	switch jobType {
	case service.JobTypeOutlineGeneration:
		return TaskOutlineGeneration, nil
	case service.JobTypeOutlineRegeneration:
		return TaskOutlineRegeneration, nil
	case service.JobTypeContentGeneration:
		return TaskContentGeneration, nil
	case service.JobTypeContentRegeneration:
		return TaskContentRegeneration, nil
	case service.JobTypeSectionRegeneration:
		return TaskSectionRegeneration, nil
	case service.JobTypeReportFileCreation:
		return TaskReportFileCreation, nil
	default:
		return "", fmt.Errorf("unsupported report job type: %s", jobType)
	}
}

// JobStateManager updates job and attempt status in the database as the worker processes tasks.
type JobStateManager interface {
	SetJobRunning(ctx context.Context, id string) error
	SetJobSucceeded(ctx context.Context, id string) error
	SetJobFailed(ctx context.Context, id, errCode, errMsg string) error
	SetAttemptRunning(ctx context.Context, attemptID string) error
	SetAttemptSucceeded(ctx context.Context, attemptID string) error
	SetAttemptFailed(ctx context.Context, attemptID, errCode, errMsg string) error
}

type ReportFileExecutor interface {
	ExecuteReportFileCreation(ctx context.Context, payload service.ReportFileExecutionPayload) error
}

type Worker struct {
	server             *asynq.Server
	mux                *asynq.ServeMux
	logger             *slog.Logger
	jobsMgr            JobStateManager
	reportFileExecutor ReportFileExecutor
}

func New(redisAddr string, logger *slog.Logger, mgr JobStateManager, reportFileExecutor ReportFileExecutor) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 4,
			Queues:      map[string]int{"document": 1},
		},
	)
	mux := asynq.NewServeMux()
	w := &Worker{server: srv, mux: mux, logger: logger, jobsMgr: mgr, reportFileExecutor: reportFileExecutor}
	mux.HandleFunc(TaskOutlineGeneration, w.handleReportJob)
	mux.HandleFunc(TaskOutlineRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskContentGeneration, w.handleReportJob)
	mux.HandleFunc(TaskContentRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskSectionRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskReportFileCreation, w.handleReportJob)
	return w
}

func (w *Worker) Start() error {
	return w.server.Start(w.mux)
}

func (w *Worker) Stop() {
	w.server.Shutdown()
}

func (w *Worker) handleReportJob(ctx context.Context, t *asynq.Task) error {
	var payload ReportJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	w.logger.InfoContext(ctx, "report job started", "job_id", payload.JobID, "attempt_id", payload.AttemptID, "job_type", payload.JobType)

	if err := w.jobsMgr.SetJobRunning(ctx, payload.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job running failed", "job_id", payload.JobID, "error", err)
	} else {
		w.recordJobStatusOperation(ctx, payload, service.OperationReportJobRunning, service.OperationResultSucceeded, "")
	}
	if payload.AttemptID != "" {
		if err := w.jobsMgr.SetAttemptRunning(ctx, payload.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt running failed", "attempt_id", payload.AttemptID, "error", err)
		}
	}

	if payload.JobType == string(service.JobTypeReportFileCreation) {
		if w.reportFileExecutor == nil {
			err := fmt.Errorf("report file executor is not configured")
			w.markFailed(ctx, payload, "executor_not_configured", err)
			return err
		}
		if err := w.reportFileExecutor.ExecuteReportFileCreation(ctx, service.ReportFileExecutionPayload{
			RequestID: payload.RequestID,
			JobID:     payload.JobID,
			UserID:    payload.UserID,
		}); err != nil {
			w.markFailed(ctx, payload, "execution_failed", err)
			return err
		}
	} else {
		// Domain execution is a placeholder until AI generation workflows land.
		w.logger.InfoContext(ctx, "report job placeholder completed", "job_id", payload.JobID, "job_type", payload.JobType)
	}

	w.logger.InfoContext(ctx, "report job completed", "job_id", payload.JobID, "job_type", payload.JobType)

	if err := w.jobsMgr.SetJobSucceeded(ctx, payload.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job succeeded failed", "job_id", payload.JobID, "error", err)
		if payload.AttemptID != "" {
			_ = w.jobsMgr.SetAttemptFailed(ctx, payload.AttemptID, "state_error", err.Error())
		}
		w.recordJobStatusOperation(ctx, payload, service.OperationReportJobFailed, service.OperationResultFailed, "failed to mark job succeeded")
		return err
	}
	if payload.AttemptID != "" {
		if err := w.jobsMgr.SetAttemptSucceeded(ctx, payload.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt succeeded failed", "attempt_id", payload.AttemptID, "error", err)
		}
	}
	w.recordJobStatusOperation(ctx, payload, service.OperationReportJobSucceeded, service.OperationResultSucceeded, "")
	return nil
}

func (w *Worker) recordJobStatusOperation(ctx context.Context, payload ReportJobPayload, operationType, result, errorMessage string) {
	recorder, ok := w.jobsMgr.(service.OperationLogRecorder)
	if !ok {
		return
	}
	_, _ = recorder.CreateOperationLog(ctx, service.OperationLog{
		OperatorID:      payload.UserID,
		OperatorName:    payload.UserID,
		OperationType:   operationType,
		TargetType:      "job",
		TargetID:        payload.JobID,
		RequestID:       payload.RequestID,
		RequestSource:   "worker",
		OperationResult: result,
		ErrorMessage:    errorMessage,
		ParameterSummary: map[string]any{
			"jobType": payload.JobType,
		},
	})
}

func (w *Worker) markFailed(ctx context.Context, payload ReportJobPayload, code string, err error) {
	w.logger.ErrorContext(ctx, "report job execution failed", "job_id", payload.JobID, "attempt_id", payload.AttemptID, "job_type", payload.JobType, "error", err)
	if failErr := w.jobsMgr.SetJobFailed(ctx, payload.JobID, code, "report job execution failed"); failErr != nil {
		w.logger.ErrorContext(ctx, "mark job failed failed", "job_id", payload.JobID, "error", failErr)
	}
	if payload.AttemptID != "" {
		if failErr := w.jobsMgr.SetAttemptFailed(ctx, payload.AttemptID, code, "report job execution failed"); failErr != nil {
			w.logger.ErrorContext(ctx, "mark attempt failed failed", "attempt_id", payload.AttemptID, "error", failErr)
		}
	}
	w.recordJobStatusOperation(ctx, payload, service.OperationReportJobFailed, service.OperationResultFailed, "report job execution failed")
}
