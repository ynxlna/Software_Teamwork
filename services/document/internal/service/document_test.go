package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateReportTemplateValidatesAndMapsDependencies(t *testing.T) {
	ctx := context.Background()
	reqCtx := RequestContext{UserID: "usr_test", RequestID: "req_test"}

	tests := []struct {
		name      string
		reqCtx    RequestContext
		input     CreateReportTemplateInput
		repo      *fakeRepository
		files     *fakeFileClient
		wantCode  Code
		fileCalls int
	}{
		{
			name:     "requires gateway user context",
			reqCtx:   RequestContext{},
			input:    validTemplateInput(),
			repo:     &fakeRepository{reportTypeExists: true},
			files:    &fakeFileClient{},
			wantCode: CodeUnauthorized,
		},
		{
			name:     "rejects non docx template",
			reqCtx:   reqCtx,
			input:    templateInputWithFile("template.pdf"),
			repo:     &fakeRepository{reportTypeExists: true},
			files:    &fakeFileClient{},
			wantCode: CodeValidation,
		},
		{
			name:      "rejects missing report type",
			reqCtx:    reqCtx,
			input:     validTemplateInput(),
			repo:      &fakeRepository{reportTypeExists: false},
			files:     &fakeFileClient{},
			wantCode:  CodeValidation,
			fileCalls: 0,
		},
		{
			name:     "maps file service failures to dependency error",
			reqCtx:   reqCtx,
			input:    validTemplateInput(),
			repo:     &fakeRepository{reportTypeExists: true},
			files:    &fakeFileClient{createErr: errors.New("file service down")},
			wantCode: CodeDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(tt.repo, tt.files)
			_, err := svc.CreateReportTemplate(ctx, tt.reqCtx, tt.input)
			if err == nil {
				t.Fatal("CreateReportTemplate() error = nil")
			}
			if code := errorCode(t, err); code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", code, tt.wantCode)
			}
			if tt.fileCalls != 0 && tt.files.createCalls != tt.fileCalls {
				t.Fatalf("file create calls = %d, want %d", tt.files.createCalls, tt.fileCalls)
			}
			if tt.fileCalls == 0 && tt.name == "rejects missing report type" && tt.files.createCalls != 0 {
				t.Fatalf("file create should not be called when report type is invalid")
			}
		})
	}
}

func TestCreateReportTemplateStoresFileMetadataAndCleansUpOnInsertFailure(t *testing.T) {
	ctx := context.Background()
	reqCtx := RequestContext{UserID: "usr_test", RequestID: "req_test"}
	now := time.Date(2026, 6, 29, 14, 30, 0, 0, time.UTC)

	repo := &fakeRepository{reportTypeExists: true}
	files := &fakeFileClient{
		file: FileObject{
			ID:        "file_internal_template",
			Filename:  "template.docx",
			SizeBytes: 32,
		},
	}
	svc := New(repo, files)
	svc.now = func() time.Time { return now }

	template, err := svc.CreateReportTemplate(ctx, reqCtx, validTemplateInput())
	if err != nil {
		t.Fatalf("CreateReportTemplate() error = %v", err)
	}
	if template.FileRef != "file_internal_template" || template.CreatedBy != "usr_test" {
		t.Fatalf("unexpected template metadata: %+v", template)
	}
	if repo.createdTemplate.FileRef != "file_internal_template" {
		t.Fatalf("repository FileRef = %q", repo.createdTemplate.FileRef)
	}
	if string(repo.createdStructure.OutlineSchema) != "[]" || string(repo.createdStructure.StyleConfig) != "{}" {
		t.Fatalf("unexpected default structure: %+v", repo.createdStructure)
	}
	if files.deleteCalls != 0 {
		t.Fatalf("DeleteFile calls = %d, want 0", files.deleteCalls)
	}

	repo = &fakeRepository{reportTypeExists: true, createTemplateErr: errors.New("db failed")}
	files = &fakeFileClient{file: FileObject{ID: "file_to_cleanup", Filename: "template.docx", SizeBytes: 32}}
	svc = New(repo, files)
	_, err = svc.CreateReportTemplate(ctx, reqCtx, validTemplateInput())
	if err == nil {
		t.Fatal("CreateReportTemplate() error = nil")
	}
	if code := errorCode(t, err); code != CodeDependency {
		t.Fatalf("error code = %q, want %q", code, CodeDependency)
	}
	if files.deleteCalls != 1 || files.deletedFileID != "file_to_cleanup" {
		t.Fatalf("cleanup delete calls/id = %d/%q", files.deleteCalls, files.deletedFileID)
	}
}

func TestUpdateReportTemplateStructureValidatesShape(t *testing.T) {
	ctx := context.Background()
	reqCtx := RequestContext{UserID: "usr_test"}
	repo := &fakeRepository{}
	svc := New(repo, &fakeFileClient{})

	_, err := svc.UpdateReportTemplateStructure(ctx, reqCtx, UpdateReportTemplateStructureInput{
		ID: "00000000-0000-0000-0000-000000000001",
		Structure: ReportTemplateStructure{
			OutlineSchema: []byte(`{"not":"array"}`),
			StyleConfig:   []byte(`{}`),
		},
	})
	if err == nil {
		t.Fatal("UpdateReportTemplateStructure() error = nil")
	}
	if code := errorCode(t, err); code != CodeValidation {
		t.Fatalf("error code = %q, want %q", code, CodeValidation)
	}
	if repo.updateStructureCalls != 0 {
		t.Fatalf("repository should not be called on invalid structure")
	}

	updated, err := svc.UpdateReportTemplateStructure(ctx, reqCtx, UpdateReportTemplateStructureInput{
		ID:        "00000000-0000-0000-0000-000000000001",
		Structure: ReportTemplateStructure{},
	})
	if err != nil {
		t.Fatalf("UpdateReportTemplateStructure() error = %v", err)
	}
	if string(updated.OutlineSchema) != "[]" || string(updated.StyleConfig) != "{}" {
		t.Fatalf("normalized structure = %+v", updated)
	}
}

func TestCreateReportMaterialCleansTagsAndStoresFileReference(t *testing.T) {
	ctx := context.Background()
	reqCtx := RequestContext{UserID: "usr_test"}
	repo := &fakeRepository{}
	files := &fakeFileClient{file: FileObject{ID: "file_internal_material", Filename: "material.png", SizeBytes: 16}}
	svc := New(repo, files)

	material, err := svc.CreateReportMaterial(ctx, reqCtx, CreateReportMaterialInput{
		MaterialName: " name ",
		MaterialType: " image ",
		Tags:         []string{" coal ", "audit", "coal", ""},
		File: UploadedFile{
			Filename:  "material.png",
			SizeBytes: 16,
			Content:   bytes.NewReader([]byte("material")),
		},
	})
	if err != nil {
		t.Fatalf("CreateReportMaterial() error = %v", err)
	}
	if material.FileRef != "file_internal_material" {
		t.Fatalf("FileRef = %q", material.FileRef)
	}
	if got := repo.createdMaterial.Tags; len(got) != 2 || got[0] != "coal" || got[1] != "audit" {
		t.Fatalf("cleaned tags = %#v", got)
	}
}

func TestCreateReportMaterialCleansUpFileOnInsertFailure(t *testing.T) {
	ctx := context.Background()
	reqCtx := RequestContext{UserID: "usr_test"}
	repo := &fakeRepository{createMaterialErr: errors.New("db failed")}
	files := &fakeFileClient{file: FileObject{ID: "file_to_cleanup", Filename: "material.png", SizeBytes: 16}}
	svc := New(repo, files)

	_, err := svc.CreateReportMaterial(ctx, reqCtx, CreateReportMaterialInput{
		MaterialName: "material",
		MaterialType: "image",
		File: UploadedFile{
			Filename:  "material.png",
			SizeBytes: 16,
			Content:   bytes.NewReader([]byte("material")),
		},
	})
	if err == nil {
		t.Fatal("CreateReportMaterial() error = nil")
	}
	if code := errorCode(t, err); code != CodeDependency {
		t.Fatalf("error code = %q, want %q", code, CodeDependency)
	}
	if files.deleteCalls != 1 || files.deletedFileID != "file_to_cleanup" {
		t.Fatalf("cleanup delete calls/id = %d/%q", files.deleteCalls, files.deletedFileID)
	}
}

func validTemplateInput() CreateReportTemplateInput {
	return templateInputWithFile("template.docx")
}

func templateInputWithFile(filename string) CreateReportTemplateInput {
	return CreateReportTemplateInput{
		TemplateName: "template",
		ReportType:   "summer_peak_inspection",
		File: UploadedFile{
			Filename:  filename,
			SizeBytes: 32,
			Content:   bytes.NewReader([]byte("template")),
		},
	}
}

func errorCode(t *testing.T, err error) Code {
	t.Helper()
	appErr, ok := Classify(err)
	if !ok {
		t.Fatalf("error is not classified: %v", err)
	}
	return appErr.Code
}

type fakeRepository struct {
	reportTypeExists     bool
	reportTypeErr        error
	createTemplateErr    error
	createdTemplate      ReportTemplate
	createdStructure     ReportTemplateStructure
	createMaterialErr    error
	createdMaterial      ReportMaterial
	updateStructureCalls int
}

func (r *fakeRepository) ListReportTypes(context.Context) ([]ReportType, error) {
	return nil, nil
}

func (r *fakeRepository) ReportTypeExists(context.Context, string) (bool, error) {
	return r.reportTypeExists, r.reportTypeErr
}

func (r *fakeRepository) ListReportTemplates(context.Context, ReportTemplateListFilter) (ReportTemplateListResult, error) {
	return ReportTemplateListResult{}, nil
}

func (r *fakeRepository) CreateReportTemplate(_ context.Context, value ReportTemplate, structure ReportTemplateStructure) (ReportTemplate, error) {
	r.createdTemplate = value
	r.createdStructure = structure
	if r.createTemplateErr != nil {
		return ReportTemplate{}, r.createTemplateErr
	}
	return value, nil
}

func (r *fakeRepository) FindReportTemplateByID(context.Context, string) (ReportTemplate, error) {
	return ReportTemplate{}, nil
}

func (r *fakeRepository) UpdateReportTemplate(context.Context, UpdateReportTemplateInput) (ReportTemplate, error) {
	return ReportTemplate{}, nil
}

func (r *fakeRepository) DeleteReportTemplate(context.Context, string, time.Time) error {
	return nil
}

func (r *fakeRepository) GetReportTemplateStructure(context.Context, string) (ReportTemplateStructure, error) {
	return ReportTemplateStructure{}, nil
}

func (r *fakeRepository) UpdateReportTemplateStructure(_ context.Context, _ string, structure ReportTemplateStructure, _ time.Time) (ReportTemplateStructure, error) {
	r.updateStructureCalls++
	return structure, nil
}

func (r *fakeRepository) ListReportMaterials(context.Context, ReportMaterialListFilter) (ReportMaterialListResult, error) {
	return ReportMaterialListResult{}, nil
}

func (r *fakeRepository) CreateReportMaterial(_ context.Context, value ReportMaterial) (ReportMaterial, error) {
	r.createdMaterial = value
	if r.createMaterialErr != nil {
		return ReportMaterial{}, r.createMaterialErr
	}
	return value, nil
}

func (r *fakeRepository) FindReportMaterialByID(context.Context, string) (ReportMaterial, error) {
	return ReportMaterial{}, nil
}

func (r *fakeRepository) DeleteReportMaterial(context.Context, string, time.Time) error {
	return nil
}

type fakeFileClient struct {
	file          FileObject
	createErr     error
	createCalls   int
	deleteCalls   int
	deletedFileID string
}

func (c *fakeFileClient) CreateFile(context.Context, RequestContext, UploadedFile) (FileObject, error) {
	c.createCalls++
	if c.createErr != nil {
		return FileObject{}, c.createErr
	}
	if c.file.ID == "" {
		c.file = FileObject{ID: "file_internal", Filename: "file.bin", SizeBytes: 1}
	}
	return c.file, nil
}

func (c *fakeFileClient) DeleteFile(_ context.Context, _ RequestContext, fileID string) error {
	c.deleteCalls++
	c.deletedFileID = fileID
	return nil
}
