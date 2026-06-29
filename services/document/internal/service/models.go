package service

import (
	"encoding/json"
	"io"
	"time"
)

type ReportStatus string

const (
	ReportStatusDraft ReportStatus = "draft"
)

type JobType string

const (
	JobTypeOutlineGeneration JobType = "outline_generation"
)

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
)

type ReportType struct {
	Code              string
	Name              string
	Description       string
	Enabled           bool
	DefaultTemplateID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PageMeta struct {
	Page     int
	PageSize int
	Total    int
}

type RequestContext struct {
	RequestID      string
	UserID         string
	CallerService  string
	ServiceToken   string
	Roles          []string
	Permissions    []string
	ForwardedFor   string
	ForwardedProto string
}

type UploadedFile struct {
	Filename       string
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	Content        io.Reader
}

type FileObject struct {
	ID             string
	Filename       string
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	CreatedAt      time.Time
}

type ReportTemplate struct {
	ID           string
	TemplateName string
	ReportType   string
	Version      int
	FileRef      string
	Filename     string
	FileSize     int64
	Description  string
	Enabled      bool
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type ReportTemplateStructure struct {
	OutlineSchema json.RawMessage
	StyleConfig   json.RawMessage
}

type ReportTemplateListFilter struct {
	Page       int
	PageSize   int
	ReportType string
	Enabled    *bool
}

type ReportTemplateListResult struct {
	Items []ReportTemplate
	Page  PageMeta
}

type CreateReportTemplateInput struct {
	TemplateName string
	ReportType   string
	Description  string
	File         UploadedFile
}

type UpdateReportTemplateInput struct {
	ID           string
	TemplateName *string
	Description  *string
	Enabled      *bool
}

type UpdateReportTemplateStructureInput struct {
	ID        string
	Structure ReportTemplateStructure
}

type ReportMaterial struct {
	ID           string
	MaterialName string
	MaterialType string
	Category     string
	FileRef      string
	Filename     string
	FileSize     int64
	Description  string
	Tags         []string
	Enabled      bool
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type ReportMaterialListFilter struct {
	Page     int
	PageSize int
	Category string
	Enabled  *bool
}

type ReportMaterialListResult struct {
	Items []ReportMaterial
	Page  PageMeta
}

type CreateReportMaterialInput struct {
	MaterialName string
	MaterialType string
	Category     string
	Description  string
	Tags         []string
	File         UploadedFile
}

type Report struct {
	ID                 string
	Name               string
	ReportType         string
	TemplateID         string
	Topic              string
	Specialty          string
	BusinessObject     string
	Year               int
	Status             ReportStatus
	CreatorID          string
	CreatorName        string
	Source             string
	LatestJobID        string
	LatestReportFileID string
	GeneratedAt        *time.Time
	ExportedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

type ReportJob struct {
	ID           string
	RequestID    string
	Source       string
	JobType      JobType
	TargetType   string
	TargetID     string
	AsynqTaskID  string
	QueueName    string
	ReportID     string
	TemplateID   string
	Status       JobStatus
	ErrorCode    string
	ErrorMessage string
	RetryCount   int
	MaxAttempts  int
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
}

type ReportJobAttempt struct {
	ID            string
	JobID         string
	AttemptNumber int
	AsynqTaskID   string
	TriggerSource string
	Reason        string
	Status        JobStatus
	ErrorCode     string
	ErrorMessage  string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
}

type ReportEvent struct {
	ID        string
	ReportID  string
	JobID     string
	EventType string
	Message   string
	CreatedAt time.Time
}
