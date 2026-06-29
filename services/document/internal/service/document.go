package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Repository interface {
	ListReportTypes(ctx context.Context) ([]ReportType, error)
	ReportTypeExists(ctx context.Context, code string) (bool, error)
	ListReportTemplates(ctx context.Context, filter ReportTemplateListFilter) (ReportTemplateListResult, error)
	CreateReportTemplate(ctx context.Context, value ReportTemplate, structure ReportTemplateStructure) (ReportTemplate, error)
	FindReportTemplateByID(ctx context.Context, id string) (ReportTemplate, error)
	UpdateReportTemplate(ctx context.Context, input UpdateReportTemplateInput) (ReportTemplate, error)
	DeleteReportTemplate(ctx context.Context, id string, deletedAt time.Time) error
	GetReportTemplateStructure(ctx context.Context, id string) (ReportTemplateStructure, error)
	UpdateReportTemplateStructure(ctx context.Context, id string, structure ReportTemplateStructure, updatedAt time.Time) (ReportTemplateStructure, error)
	ListReportMaterials(ctx context.Context, filter ReportMaterialListFilter) (ReportMaterialListResult, error)
	CreateReportMaterial(ctx context.Context, value ReportMaterial) (ReportMaterial, error)
	FindReportMaterialByID(ctx context.Context, id string) (ReportMaterial, error)
	DeleteReportMaterial(ctx context.Context, id string, deletedAt time.Time) error
}

type FileClient interface {
	CreateFile(ctx context.Context, reqCtx RequestContext, file UploadedFile) (FileObject, error)
	DeleteFile(ctx context.Context, reqCtx RequestContext, fileID string) error
}

type Service struct {
	repo  Repository
	files FileClient
	now   func() time.Time
}

func New(repo Repository, files FileClient) *Service {
	return &Service{
		repo:  repo,
		files: files,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) ListReportTypes(ctx context.Context, reqCtx RequestContext) ([]ReportType, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return nil, err
	}
	types, err := s.repo.ListReportTypes(ctx)
	if err != nil {
		return nil, dependencyError("list report types", err)
	}
	return types, nil
}

func (s *Service) ListReportTemplates(ctx context.Context, reqCtx RequestContext, filter ReportTemplateListFilter) (ReportTemplateListResult, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplateListResult{}, err
	}
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	result, err := s.repo.ListReportTemplates(ctx, filter)
	if err != nil {
		return ReportTemplateListResult{}, dependencyError("list report templates", err)
	}
	return result, nil
}

func (s *Service) CreateReportTemplate(ctx context.Context, reqCtx RequestContext, input CreateReportTemplateInput) (ReportTemplate, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplate{}, err
	}
	if err := validateCreateTemplate(input); err != nil {
		return ReportTemplate{}, err
	}
	exists, err := s.repo.ReportTypeExists(ctx, input.ReportType)
	if err != nil {
		return ReportTemplate{}, dependencyError("check report type", err)
	}
	if !exists {
		return ReportTemplate{}, ValidationError(map[string]string{"reportType": "does not exist or is disabled"})
	}
	file, err := s.files.CreateFile(ctx, reqCtx, input.File)
	if err != nil {
		return ReportTemplate{}, mapFileError(err)
	}

	now := s.now()
	created, err := s.repo.CreateReportTemplate(ctx, ReportTemplate{
		ID:           newID(),
		TemplateName: strings.TrimSpace(input.TemplateName),
		ReportType:   strings.TrimSpace(input.ReportType),
		Version:      1,
		FileRef:      file.ID,
		Filename:     file.Filename,
		FileSize:     file.SizeBytes,
		Description:  strings.TrimSpace(input.Description),
		Enabled:      true,
		CreatedBy:    reqCtx.UserID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, defaultTemplateStructure())
	if err != nil {
		_ = s.files.DeleteFile(context.WithoutCancel(ctx), reqCtx, file.ID)
		return ReportTemplate{}, dependencyError("create report template", err)
	}
	return created, nil
}

func (s *Service) GetReportTemplate(ctx context.Context, reqCtx RequestContext, id string) (ReportTemplate, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplate{}, err
	}
	template, err := s.repo.FindReportTemplateByID(ctx, id)
	if err != nil {
		return ReportTemplate{}, mapRepositoryReadError(err, "report template not found")
	}
	return template, nil
}

func (s *Service) UpdateReportTemplate(ctx context.Context, reqCtx RequestContext, input UpdateReportTemplateInput) (ReportTemplate, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplate{}, err
	}
	fields := map[string]string{}
	if strings.TrimSpace(input.ID) == "" {
		fields["reportTemplateId"] = "is required"
	}
	if input.TemplateName != nil && strings.TrimSpace(*input.TemplateName) == "" {
		fields["templateName"] = "must not be empty"
	}
	if len(fields) > 0 {
		return ReportTemplate{}, ValidationError(fields)
	}
	template, err := s.repo.UpdateReportTemplate(ctx, input)
	if err != nil {
		return ReportTemplate{}, mapRepositoryReadError(err, "report template not found")
	}
	return template, nil
}

func (s *Service) DeleteReportTemplate(ctx context.Context, reqCtx RequestContext, id string) error {
	if err := requireGatewayContext(reqCtx); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ValidationError(map[string]string{"reportTemplateId": "is required"})
	}
	if err := s.repo.DeleteReportTemplate(ctx, id, s.now()); err != nil {
		return mapRepositoryReadError(err, "report template not found")
	}
	return nil
}

func (s *Service) GetReportTemplateStructure(ctx context.Context, reqCtx RequestContext, id string) (ReportTemplateStructure, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplateStructure{}, err
	}
	structure, err := s.repo.GetReportTemplateStructure(ctx, id)
	if err != nil {
		return ReportTemplateStructure{}, mapRepositoryReadError(err, "report template not found")
	}
	return normalizeStructure(structure), nil
}

func (s *Service) UpdateReportTemplateStructure(ctx context.Context, reqCtx RequestContext, input UpdateReportTemplateStructureInput) (ReportTemplateStructure, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportTemplateStructure{}, err
	}
	if strings.TrimSpace(input.ID) == "" {
		return ReportTemplateStructure{}, ValidationError(map[string]string{"reportTemplateId": "is required"})
	}
	structure, err := validateTemplateStructure(input.Structure)
	if err != nil {
		return ReportTemplateStructure{}, err
	}
	updated, err := s.repo.UpdateReportTemplateStructure(ctx, input.ID, structure, s.now())
	if err != nil {
		return ReportTemplateStructure{}, mapRepositoryReadError(err, "report template not found")
	}
	return normalizeStructure(updated), nil
}

func (s *Service) ListReportMaterials(ctx context.Context, reqCtx RequestContext, filter ReportMaterialListFilter) (ReportMaterialListResult, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportMaterialListResult{}, err
	}
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	result, err := s.repo.ListReportMaterials(ctx, filter)
	if err != nil {
		return ReportMaterialListResult{}, dependencyError("list report materials", err)
	}
	return result, nil
}

func (s *Service) CreateReportMaterial(ctx context.Context, reqCtx RequestContext, input CreateReportMaterialInput) (ReportMaterial, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportMaterial{}, err
	}
	if err := validateCreateMaterial(input); err != nil {
		return ReportMaterial{}, err
	}
	file, err := s.files.CreateFile(ctx, reqCtx, input.File)
	if err != nil {
		return ReportMaterial{}, mapFileError(err)
	}
	now := s.now()
	created, err := s.repo.CreateReportMaterial(ctx, ReportMaterial{
		ID:           newID(),
		MaterialName: strings.TrimSpace(input.MaterialName),
		MaterialType: strings.TrimSpace(input.MaterialType),
		Category:     strings.TrimSpace(input.Category),
		FileRef:      file.ID,
		Filename:     file.Filename,
		FileSize:     file.SizeBytes,
		Description:  strings.TrimSpace(input.Description),
		Tags:         cleanList(input.Tags),
		Enabled:      true,
		CreatedBy:    reqCtx.UserID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		_ = s.files.DeleteFile(context.WithoutCancel(ctx), reqCtx, file.ID)
		return ReportMaterial{}, dependencyError("create report material", err)
	}
	return created, nil
}

func (s *Service) GetReportMaterial(ctx context.Context, reqCtx RequestContext, id string) (ReportMaterial, error) {
	if err := requireGatewayContext(reqCtx); err != nil {
		return ReportMaterial{}, err
	}
	material, err := s.repo.FindReportMaterialByID(ctx, id)
	if err != nil {
		return ReportMaterial{}, mapRepositoryReadError(err, "report material not found")
	}
	return material, nil
}

func (s *Service) DeleteReportMaterial(ctx context.Context, reqCtx RequestContext, id string) error {
	if err := requireGatewayContext(reqCtx); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ValidationError(map[string]string{"materialId": "is required"})
	}
	if err := s.repo.DeleteReportMaterial(ctx, id, s.now()); err != nil {
		return mapRepositoryReadError(err, "report material not found")
	}
	return nil
}

func validateCreateTemplate(input CreateReportTemplateInput) error {
	fields := map[string]string{}
	if strings.TrimSpace(input.TemplateName) == "" {
		fields["templateName"] = "is required"
	}
	if strings.TrimSpace(input.ReportType) == "" {
		fields["reportType"] = "is required"
	}
	validateUpload(fields, input.File)
	if name := strings.TrimSpace(input.File.Filename); name != "" && strings.ToLower(filepath.Ext(name)) != ".docx" {
		fields["file"] = "must be a DOCX template"
	}
	if len(fields) > 0 {
		return ValidationError(fields)
	}
	return nil
}

func validateCreateMaterial(input CreateReportMaterialInput) error {
	fields := map[string]string{}
	if strings.TrimSpace(input.MaterialName) == "" {
		fields["materialName"] = "is required"
	}
	if strings.TrimSpace(input.MaterialType) == "" {
		fields["materialType"] = "is required"
	}
	validateUpload(fields, input.File)
	if len(fields) > 0 {
		return ValidationError(fields)
	}
	return nil
}

func validateUpload(fields map[string]string, file UploadedFile) {
	if file.Content == nil {
		fields["file"] = "is required"
		return
	}
	if strings.TrimSpace(file.Filename) == "" {
		fields["file"] = "filename is required"
	}
	if file.SizeBytes == 0 {
		fields["file"] = "must not be empty"
	}
}

func validateTemplateStructure(structure ReportTemplateStructure) (ReportTemplateStructure, error) {
	structure = normalizeStructure(structure)
	fields := map[string]string{}
	if !jsonIsArray(structure.OutlineSchema) {
		fields["outlineSchema"] = "must be an array"
	}
	if !jsonIsObject(structure.StyleConfig) {
		fields["styleConfig"] = "must be an object"
	}
	if len(fields) > 0 {
		return ReportTemplateStructure{}, ValidationError(fields)
	}
	return structure, nil
}

func normalizeStructure(structure ReportTemplateStructure) ReportTemplateStructure {
	if len(structure.OutlineSchema) == 0 || string(structure.OutlineSchema) == "null" || string(structure.OutlineSchema) == "{}" {
		structure.OutlineSchema = json.RawMessage("[]")
	}
	if len(structure.StyleConfig) == 0 || string(structure.StyleConfig) == "null" || string(structure.StyleConfig) == "[]" {
		structure.StyleConfig = json.RawMessage("{}")
	}
	return structure
}

func defaultTemplateStructure() ReportTemplateStructure {
	return ReportTemplateStructure{
		OutlineSchema: json.RawMessage("[]"),
		StyleConfig:   json.RawMessage("{}"),
	}
}

func jsonIsArray(raw json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	_, ok := value.([]any)
	return ok
}

func jsonIsObject(raw json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	_, ok := value.(map[string]any)
	return ok
}

func requireGatewayContext(reqCtx RequestContext) error {
	if strings.TrimSpace(reqCtx.UserID) == "" {
		return NewError(CodeUnauthorized, "authentication is required", nil)
	}
	return nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

func cleanList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func mapFileError(err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Code == CodeValidation {
		return err
	}
	return dependencyError("file service failed", err)
}

func mapRepositoryReadError(err error, notFoundMessage string) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return err
	}
	return dependencyError(notFoundMessage, err)
}

func dependencyError(message string, err error) *AppError {
	return NewError(CodeDependency, message, err)
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
