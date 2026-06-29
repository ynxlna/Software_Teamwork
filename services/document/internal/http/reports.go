package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

// ReportService is the subset of *service.ReportService consumed by the
// HTTP layer. Declaring an interface here lets tests inject a fake without
// standing up PostgreSQL.
type ReportService interface {
	ListReports(ctx context.Context, reqCtx service.RequestContext, filter service.ReportListFilter) (service.ReportListResult, error)
	CreateReport(ctx context.Context, reqCtx service.RequestContext, input service.CreateReportInput) (service.Report, error)
	GetReport(ctx context.Context, reqCtx service.RequestContext, reportID string) (service.Report, error)
	UpdateReport(ctx context.Context, reqCtx service.RequestContext, reportID string, input service.UpdateReportInput) (service.Report, error)
	SoftDeleteReport(ctx context.Context, reqCtx service.RequestContext, reportID string) error

	ListOutlines(ctx context.Context, reqCtx service.RequestContext, reportID string) ([]service.ReportOutline, error)
	CreateOutline(ctx context.Context, reqCtx service.RequestContext, reportID string, input service.CreateOutlineInput) (service.ReportOutline, error)
	GetOutline(ctx context.Context, reqCtx service.RequestContext, reportID, outlineID string) (service.ReportOutline, error)
	UpdateOutline(ctx context.Context, reqCtx service.RequestContext, reportID, outlineID string, input service.UpdateOutlineInput) (service.ReportOutline, error)
	DeleteOutlineSection(ctx context.Context, reqCtx service.RequestContext, reportID, outlineID, sectionID string) (service.ReportOutline, error)

	ListSections(ctx context.Context, reqCtx service.RequestContext, reportID string) ([]service.ReportSection, error)
	CreateSection(ctx context.Context, reqCtx service.RequestContext, reportID string, input service.CreateSectionInput) (service.ReportSection, error)
	SaveSections(ctx context.Context, reqCtx service.RequestContext, reportID string, input service.SaveSectionsInput) ([]service.ReportSection, error)
	GetSection(ctx context.Context, reqCtx service.RequestContext, reportID, sectionID string) (service.ReportSection, error)
	UpdateSection(ctx context.Context, reqCtx service.RequestContext, reportID, sectionID string, input service.UpdateSectionInput) (service.ReportSection, error)

	ListSectionVersions(ctx context.Context, reqCtx service.RequestContext, reportID, sectionID string) ([]service.ReportSectionVersion, error)
	CreateSectionVersion(ctx context.Context, reqCtx service.RequestContext, reportID, sectionID string, input service.CreateSectionVersionInput) (service.ReportSectionVersion, error)
}

// registerReportRoutes wires up the reports / outlines / sections resource
// routes. Paths follow the same flat, unprefixed convention as the
// report-templates / report-materials routes registered in routes().
func (s *Server) registerReportRoutes() {
	s.mux.HandleFunc("GET /reports", s.handleListReports)
	s.mux.HandleFunc("POST /reports", s.handleCreateReport)
	s.mux.HandleFunc("GET /reports/{reportId}", s.handleGetReport)
	s.mux.HandleFunc("PATCH /reports/{reportId}", s.handleUpdateReport)
	s.mux.HandleFunc("DELETE /reports/{reportId}", s.handleDeleteReport)

	s.mux.HandleFunc("GET /reports/{reportId}/outlines", s.handleListOutlines)
	s.mux.HandleFunc("POST /reports/{reportId}/outlines", s.handleCreateOutline)
	s.mux.HandleFunc("GET /reports/{reportId}/outlines/{outlineId}", s.handleGetOutline)
	s.mux.HandleFunc("PATCH /reports/{reportId}/outlines/{outlineId}", s.handleUpdateOutline)
	s.mux.HandleFunc("DELETE /reports/{reportId}/outlines/{outlineId}/sections/{sectionId}", s.handleDeleteOutlineSection)

	s.mux.HandleFunc("GET /reports/{reportId}/sections", s.handleListSections)
	s.mux.HandleFunc("POST /reports/{reportId}/sections", s.handleCreateSection)
	s.mux.HandleFunc("GET /reports/{reportId}/sections/{sectionId}", s.handleGetSection)
	s.mux.HandleFunc("PATCH /reports/{reportId}/sections/{sectionId}", s.handleUpdateSection)
	s.mux.HandleFunc("GET /reports/{reportId}/sections/{sectionId}/versions", s.handleListSectionVersions)
	s.mux.HandleFunc("POST /reports/{reportId}/sections/{sectionId}/versions", s.handleCreateSectionVersion)
}

func (s *Server) requireReportService(w http.ResponseWriter, r *http.Request) bool {
	if s.reportService != nil {
		return true
	}
	writeError(w, r, service.NewError(service.CodeDependency, "report service is not configured", nil))
	return false
}

// --- DTOs ---

type reportDTO struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ReportType         string  `json:"reportType"`
	TemplateID         string  `json:"templateId,omitempty"`
	Topic              string  `json:"topic"`
	Specialty          string  `json:"specialty,omitempty"`
	BusinessObject     string  `json:"businessObject,omitempty"`
	Year               int     `json:"year,omitempty"`
	Status             string  `json:"status"`
	CreatorID          string  `json:"creatorId,omitempty"`
	CreatorName        string  `json:"creatorName,omitempty"`
	Source             string  `json:"source,omitempty"`
	LatestJobID        string  `json:"latestJobId,omitempty"`
	LatestReportFileID string  `json:"latestReportFileId,omitempty"`
	GeneratedAt        *string `json:"generatedAt,omitempty"`
	ExportedAt         *string `json:"exportedAt,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

func toReportDTO(report service.Report) reportDTO {
	return reportDTO{
		ID:                 report.ID,
		Name:               report.Name,
		ReportType:         report.ReportType,
		TemplateID:         report.TemplateID,
		Topic:              report.Topic,
		Specialty:          report.Specialty,
		BusinessObject:     report.BusinessObject,
		Year:               report.Year,
		Status:             string(report.Status),
		CreatorID:          report.CreatorID,
		CreatorName:        report.CreatorName,
		Source:             report.Source,
		LatestJobID:        report.LatestJobID,
		LatestReportFileID: report.LatestReportFileID,
		GeneratedAt:        formatTimePtr(report.GeneratedAt),
		ExportedAt:         formatTimePtr(report.ExportedAt),
		CreatedAt:          report.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          report.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type createReportRequest struct {
	Name           string `json:"name"`
	ReportType     string `json:"reportType"`
	TemplateID     string `json:"templateId"`
	Topic          string `json:"topic"`
	Specialty      string `json:"specialty,omitempty"`
	BusinessObject string `json:"businessObject,omitempty"`
	Year           int    `json:"year,omitempty"`
	Source         string `json:"source,omitempty"`
}

type updateReportRequest struct {
	Name           *string `json:"name,omitempty"`
	TemplateID     *string `json:"templateId,omitempty"`
	Topic          *string `json:"topic,omitempty"`
	Specialty      *string `json:"specialty,omitempty"`
	BusinessObject *string `json:"businessObject,omitempty"`
	Year           *int    `json:"year,omitempty"`
}

type outlineNodeDTO struct {
	ID              string           `json:"id,omitempty"`
	ClientSectionID string           `json:"clientSectionId,omitempty"`
	Title           string           `json:"title"`
	Level           int              `json:"level,omitempty"`
	Numbering       string           `json:"numbering,omitempty"`
	Children        []outlineNodeDTO `json:"children,omitempty"`
}

func toOutlineNodeDTOs(nodes []service.ReportOutlineNode) []outlineNodeDTO {
	result := make([]outlineNodeDTO, len(nodes))
	for i, node := range nodes {
		result[i] = outlineNodeDTO{
			ID:              node.ID,
			ClientSectionID: node.ClientSectionID,
			Title:           node.Title,
			Level:           node.Level,
			Numbering:       node.Numbering,
			Children:        toOutlineNodeDTOs(node.Children),
		}
	}
	return result
}

func fromOutlineNodeDTOs(nodes []outlineNodeDTO) []service.ReportOutlineNode {
	result := make([]service.ReportOutlineNode, len(nodes))
	for i, node := range nodes {
		result[i] = service.ReportOutlineNode{
			ID:              node.ID,
			ClientSectionID: node.ClientSectionID,
			Title:           node.Title,
			Level:           node.Level,
			Numbering:       node.Numbering,
			Children:        fromOutlineNodeDTOs(node.Children),
		}
	}
	return result
}

type outlineDTO struct {
	ID           string           `json:"id"`
	ReportID     string           `json:"reportId"`
	Version      int              `json:"version"`
	Sections     []outlineNodeDTO `json:"sections"`
	SourceJobID  string           `json:"sourceJobId,omitempty"`
	ManualEdited bool             `json:"manualEdited"`
	CreatedAt    string           `json:"createdAt"`
	UpdatedAt    string           `json:"updatedAt"`
}

func toOutlineDTO(outline service.ReportOutline) outlineDTO {
	return outlineDTO{
		ID:           outline.ID,
		ReportID:     outline.ReportID,
		Version:      outline.Version,
		Sections:     toOutlineNodeDTOs(outline.Sections),
		SourceJobID:  outline.SourceJobID,
		ManualEdited: outline.ManualEdited,
		CreatedAt:    outline.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    outline.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type createOutlineRequest struct {
	Source   string           `json:"source"`
	Sections []outlineNodeDTO `json:"sections"`
}

type updateOutlineRequest struct {
	Sections     []outlineNodeDTO `json:"sections"`
	ManualEdited *bool            `json:"manualEdited,omitempty"`
}

type sectionDTO struct {
	ID               string           `json:"id"`
	ReportID         string           `json:"reportId"`
	ParentID         string           `json:"parentId,omitempty"`
	OutlineNodeID    string           `json:"outlineNodeId,omitempty"`
	Title            string           `json:"title"`
	Level            int              `json:"level"`
	SortOrder        int              `json:"sortOrder,omitempty"`
	Numbering        string           `json:"numbering,omitempty"`
	SectionType      string           `json:"sectionType,omitempty"`
	Content          string           `json:"content,omitempty"`
	Tables           []map[string]any `json:"tables,omitempty"`
	GenerationStatus string           `json:"generationStatus"`
	ContentSource    string           `json:"contentSource,omitempty"`
	ManualEdited     bool             `json:"manualEdited"`
	Version          int              `json:"version,omitempty"`
	LastJobID        string           `json:"lastJobId,omitempty"`
	GeneratedAt      *string          `json:"generatedAt,omitempty"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
}

func toSectionDTO(section service.ReportSection) sectionDTO {
	return sectionDTO{
		ID:               section.ID,
		ReportID:         section.ReportID,
		ParentID:         section.ParentID,
		OutlineNodeID:    section.OutlineNodeID,
		Title:            section.Title,
		Level:            section.Level,
		SortOrder:        section.SortOrder,
		Numbering:        section.Numbering,
		SectionType:      string(section.SectionType),
		Content:          section.Content,
		Tables:           section.Tables,
		GenerationStatus: string(section.GenerationStatus),
		ContentSource:    string(section.ContentSource),
		ManualEdited:     section.ManualEdited,
		Version:          section.Version,
		LastJobID:        section.LastJobID,
		GeneratedAt:      formatTimePtr(section.GeneratedAt),
		CreatedAt:        section.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        section.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type createSectionRequest struct {
	OutlineNodeID string           `json:"outlineNodeId,omitempty"`
	ParentID      string           `json:"parentId,omitempty"`
	Title         string           `json:"title"`
	Level         int              `json:"level,omitempty"`
	SortOrder     *int             `json:"sortOrder,omitempty"`
	Numbering     string           `json:"numbering,omitempty"`
	Content       string           `json:"content,omitempty"`
	Tables        []map[string]any `json:"tables,omitempty"`
}

type updateSectionRequest struct {
	Title        *string           `json:"title,omitempty"`
	Content      *string           `json:"content,omitempty"`
	Tables       *[]map[string]any `json:"tables,omitempty"`
	ManualEdited *bool             `json:"manualEdited,omitempty"`
}

type saveSectionRequest struct {
	ID            string            `json:"id,omitempty"`
	OutlineNodeID *string           `json:"outlineNodeId,omitempty"`
	ParentID      *string           `json:"parentId,omitempty"`
	Title         *string           `json:"title,omitempty"`
	Level         *int              `json:"level,omitempty"`
	SortOrder     *int              `json:"sortOrder,omitempty"`
	Numbering     *string           `json:"numbering,omitempty"`
	Content       *string           `json:"content,omitempty"`
	Tables        *[]map[string]any `json:"tables,omitempty"`
	ManualEdited  *bool             `json:"manualEdited,omitempty"`
}

type saveSectionsRequest struct {
	Sections []saveSectionRequest `json:"sections"`
}

func (r saveSectionRequest) toService() service.SaveSectionInput {
	return service.SaveSectionInput{
		ID:            r.ID,
		OutlineNodeID: r.OutlineNodeID,
		ParentID:      r.ParentID,
		Title:         r.Title,
		Level:         r.Level,
		SortOrder:     r.SortOrder,
		Numbering:     r.Numbering,
		Content:       r.Content,
		Tables:        r.Tables,
		ManualEdited:  r.ManualEdited,
	}
}

type sectionVersionDTO struct {
	ID        string           `json:"id"`
	ReportID  string           `json:"reportId"`
	SectionID string           `json:"sectionId"`
	Version   int              `json:"version"`
	Source    string           `json:"source"`
	Content   string           `json:"content,omitempty"`
	Tables    []map[string]any `json:"tables,omitempty"`
	JobID     string           `json:"jobId,omitempty"`
	CreatedAt string           `json:"createdAt"`
}

func toSectionVersionDTO(version service.ReportSectionVersion) sectionVersionDTO {
	return sectionVersionDTO{
		ID:        version.ID,
		ReportID:  version.ReportID,
		SectionID: version.SectionID,
		Version:   version.Version,
		Source:    string(version.Source),
		Content:   version.Content,
		Tables:    version.Tables,
		JobID:     version.JobID,
		CreatedAt: version.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type createSectionVersionRequest struct {
	Source       string            `json:"source"`
	Requirements string            `json:"requirements,omitempty"`
	Content      *string           `json:"content,omitempty"`
	Tables       *[]map[string]any `json:"tables,omitempty"`
}

// --- Reports handlers ---

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	page, pageSize, err := parsePage(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	reqCtx := s.requestContext(r)
	result, err := s.reportService.ListReports(r.Context(), reqCtx, service.ReportListFilter{
		Page:       page,
		PageSize:   pageSize,
		ReportType: r.URL.Query().Get("reportType"),
		Status:     r.URL.Query().Get("status"),
		Keyword:    r.URL.Query().Get("keyword"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	data := make([]reportDTO, len(result.Items))
	for i, report := range result.Items {
		data[i] = toReportDTO(report)
	}
	writePage(w, r, http.StatusOK, data, result.Page)
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body createReportRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	report, err := s.reportService.CreateReport(r.Context(), reqCtx, service.CreateReportInput{
		Name:           body.Name,
		ReportType:     body.ReportType,
		TemplateID:     body.TemplateID,
		Topic:          body.Topic,
		Specialty:      body.Specialty,
		BusinessObject: body.BusinessObject,
		Year:           body.Year,
		Source:         body.Source,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, toReportDTO(report))
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	report, err := s.reportService.GetReport(r.Context(), reqCtx, r.PathValue("reportId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toReportDTO(report))
}

func (s *Server) handleUpdateReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body updateReportRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	report, err := s.reportService.UpdateReport(r.Context(), reqCtx, r.PathValue("reportId"), service.UpdateReportInput{
		Name:           body.Name,
		TemplateID:     body.TemplateID,
		Topic:          body.Topic,
		Specialty:      body.Specialty,
		BusinessObject: body.BusinessObject,
		Year:           body.Year,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toReportDTO(report))
}

func (s *Server) handleDeleteReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	if err := s.reportService.SoftDeleteReport(r.Context(), reqCtx, r.PathValue("reportId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Outline handlers ---

func (s *Server) handleListOutlines(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	outlines, err := s.reportService.ListOutlines(r.Context(), reqCtx, r.PathValue("reportId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	data := make([]outlineDTO, len(outlines))
	for i, outline := range outlines {
		data[i] = toOutlineDTO(outline)
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) handleCreateOutline(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body createOutlineRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	outline, err := s.reportService.CreateOutline(r.Context(), reqCtx, r.PathValue("reportId"), service.CreateOutlineInput{
		Source:   service.OutlineSource(body.Source),
		Sections: fromOutlineNodeDTOs(body.Sections),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, toOutlineDTO(outline))
}

func (s *Server) handleGetOutline(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	outline, err := s.reportService.GetOutline(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("outlineId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toOutlineDTO(outline))
}

func (s *Server) handleUpdateOutline(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body updateOutlineRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	outline, err := s.reportService.UpdateOutline(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("outlineId"), service.UpdateOutlineInput{
		Sections:     fromOutlineNodeDTOs(body.Sections),
		ManualEdited: body.ManualEdited,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toOutlineDTO(outline))
}

func (s *Server) handleDeleteOutlineSection(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	_, err := s.reportService.DeleteOutlineSection(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("outlineId"), r.PathValue("sectionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Section handlers ---

func (s *Server) handleListSections(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	sections, err := s.reportService.ListSections(r.Context(), reqCtx, r.PathValue("reportId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	data := make([]sectionDTO, len(sections))
	for i, section := range sections {
		data[i] = toSectionDTO(section)
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) handleCreateSection(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	rawBody, ok := decodeJSONRaw(w, r)
	if !ok {
		return
	}
	reqCtx := s.requestContext(r)
	if hasSectionsField(rawBody) {
		var body saveSectionsRequest
		if !decodeJSONBytes(w, r, rawBody, &body) {
			return
		}
		input := service.SaveSectionsInput{Sections: make([]service.SaveSectionInput, len(body.Sections))}
		for i, section := range body.Sections {
			input.Sections[i] = section.toService()
		}
		sections, err := s.reportService.SaveSections(r.Context(), reqCtx, r.PathValue("reportId"), input)
		if err != nil {
			writeError(w, r, err)
			return
		}
		data := make([]sectionDTO, len(sections))
		for i, section := range sections {
			data[i] = toSectionDTO(section)
		}
		writeData(w, r, http.StatusOK, data)
		return
	}

	var body createSectionRequest
	if !decodeJSONBytes(w, r, rawBody, &body) {
		return
	}
	section, err := s.reportService.CreateSection(r.Context(), reqCtx, r.PathValue("reportId"), service.CreateSectionInput{
		OutlineNodeID: body.OutlineNodeID,
		ParentID:      body.ParentID,
		Title:         body.Title,
		Level:         body.Level,
		SortOrder:     body.SortOrder,
		Numbering:     body.Numbering,
		Content:       body.Content,
		Tables:        body.Tables,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, toSectionDTO(section))
}

func (s *Server) handleGetSection(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	section, err := s.reportService.GetSection(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("sectionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toSectionDTO(section))
}

func (s *Server) handleUpdateSection(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body updateSectionRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	section, err := s.reportService.UpdateSection(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("sectionId"), service.UpdateSectionInput{
		Title:        body.Title,
		Content:      body.Content,
		Tables:       body.Tables,
		ManualEdited: body.ManualEdited,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toSectionDTO(section))
}

func (s *Server) handleListSectionVersions(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	reqCtx := s.requestContext(r)
	versions, err := s.reportService.ListSectionVersions(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("sectionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	data := make([]sectionVersionDTO, len(versions))
	for i, version := range versions {
		data[i] = toSectionVersionDTO(version)
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) handleCreateSectionVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportService(w, r) {
		return
	}
	var body createSectionVersionRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reqCtx := s.requestContext(r)
	version, err := s.reportService.CreateSectionVersion(r.Context(), reqCtx, r.PathValue("reportId"), r.PathValue("sectionId"), service.CreateSectionVersionInput{
		Source:       service.ContentSource(body.Source),
		Requirements: body.Requirements,
		Content:      body.Content,
		Tables:       body.Tables,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, toSectionVersionDTO(version))
}

// --- shared helpers ---

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func decodeJSONRaw(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must be a valid JSON object"}))
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must contain only one JSON object"}))
		return nil, false
	}
	return raw, true
}

func decodeJSONBytes(w http.ResponseWriter, r *http.Request, raw []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must be a valid JSON object"}))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must contain only one JSON object"}))
		return false
	}
	return true
}

func hasSectionsField(raw []byte) bool {
	var probe struct {
		Sections json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return len(probe.Sections) > 0
}
