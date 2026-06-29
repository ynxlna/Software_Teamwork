package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type reportTemplateResponse struct {
	ID           string `json:"id"`
	TemplateName string `json:"templateName"`
	ReportType   string `json:"reportType"`
	Version      int    `json:"version"`
	Description  string `json:"description,omitempty"`
	Enabled      bool   `json:"enabled"`
	Filename     string `json:"filename,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	CreatedBy    string `json:"createdBy,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type reportTemplateStructureResponse struct {
	OutlineSchema json.RawMessage `json:"outlineSchema"`
	StyleConfig   json.RawMessage `json:"styleConfig"`
}

func (s *Server) handleListReportTemplates(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	page, pageSize, err := parsePage(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	enabled, err := parseOptionalBoolQuery(r, "enabled")
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.documents.ListReportTemplates(r.Context(), s.requestContext(r), service.ReportTemplateListFilter{
		Page:       page,
		PageSize:   pageSize,
		ReportType: r.URL.Query().Get("reportType"),
		Enabled:    enabled,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePage(w, r, http.StatusOK, reportTemplatesFromDomain(result.Items), result.Page)
}

func (s *Server) handleCreateReportTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	file, header, ok := s.parseMultipartFile(w, r)
	if !ok {
		return
	}
	defer file.Close()

	template, err := s.documents.CreateReportTemplate(r.Context(), s.requestContext(r), service.CreateReportTemplateInput{
		TemplateName: formValue(r, "templateName"),
		ReportType:   formValue(r, "reportType"),
		Description:  formValue(r, "description"),
		File: service.UploadedFile{
			Filename:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			SizeBytes:   header.Size,
			Content:     file,
		},
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, reportTemplateFromDomain(template))
}

func (s *Server) handleGetReportTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	template, err := s.documents.GetReportTemplate(r.Context(), s.requestContext(r), r.PathValue("reportTemplateId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportTemplateFromDomain(template))
}

func (s *Server) handleUpdateReportTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	var payload struct {
		TemplateName *string `json:"templateName"`
		Description  *string `json:"description"`
		Enabled      *bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	template, err := s.documents.UpdateReportTemplate(r.Context(), s.requestContext(r), service.UpdateReportTemplateInput{
		ID:           r.PathValue("reportTemplateId"),
		TemplateName: payload.TemplateName,
		Description:  payload.Description,
		Enabled:      payload.Enabled,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportTemplateFromDomain(template))
}

func (s *Server) handleDeleteReportTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	if err := s.documents.DeleteReportTemplate(r.Context(), s.requestContext(r), r.PathValue("reportTemplateId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetReportTemplateStructure(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	structure, err := s.documents.GetReportTemplateStructure(r.Context(), s.requestContext(r), r.PathValue("reportTemplateId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportTemplateStructureFromDomain(structure))
}

func (s *Server) handleUpdateReportTemplateStructure(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	var payload reportTemplateStructureResponse
	if !decodeJSON(w, r, &payload) {
		return
	}
	structure, err := s.documents.UpdateReportTemplateStructure(r.Context(), s.requestContext(r), service.UpdateReportTemplateStructureInput{
		ID: r.PathValue("reportTemplateId"),
		Structure: service.ReportTemplateStructure{
			OutlineSchema: payload.OutlineSchema,
			StyleConfig:   payload.StyleConfig,
		},
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportTemplateStructureFromDomain(structure))
}

func reportTemplatesFromDomain(values []service.ReportTemplate) []reportTemplateResponse {
	result := make([]reportTemplateResponse, 0, len(values))
	for _, value := range values {
		result = append(result, reportTemplateFromDomain(value))
	}
	return result
}

func reportTemplateFromDomain(value service.ReportTemplate) reportTemplateResponse {
	return reportTemplateResponse{
		ID:           value.ID,
		TemplateName: value.TemplateName,
		ReportType:   value.ReportType,
		Version:      value.Version,
		Description:  value.Description,
		Enabled:      value.Enabled,
		Filename:     value.Filename,
		FileSize:     value.FileSize,
		CreatedBy:    value.CreatedBy,
		CreatedAt:    formatTime(value.CreatedAt),
		UpdatedAt:    formatTime(value.UpdatedAt),
	}
}

func reportTemplateStructureFromDomain(value service.ReportTemplateStructure) reportTemplateStructureResponse {
	return reportTemplateStructureResponse{OutlineSchema: value.OutlineSchema, StyleConfig: value.StyleConfig}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
