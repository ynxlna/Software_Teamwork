package httpapi

import (
	"net/http"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type reportMaterialResponse struct {
	ID           string   `json:"id"`
	MaterialName string   `json:"materialName"`
	MaterialType string   `json:"materialType,omitempty"`
	Category     string   `json:"category,omitempty"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags"`
	Enabled      bool     `json:"enabled"`
	Filename     string   `json:"filename,omitempty"`
	FileSize     int64    `json:"fileSize,omitempty"`
	CreatedBy    string   `json:"createdBy,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

func (s *Server) handleListReportMaterials(w http.ResponseWriter, r *http.Request) {
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
	result, err := s.documents.ListReportMaterials(r.Context(), s.requestContext(r), service.ReportMaterialListFilter{
		Page:     page,
		PageSize: pageSize,
		Category: r.URL.Query().Get("category"),
		Enabled:  enabled,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePage(w, r, http.StatusOK, reportMaterialsFromDomain(result.Items), result.Page)
}

func (s *Server) handleCreateReportMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	file, header, ok := s.parseMultipartFile(w, r)
	if !ok {
		return
	}
	defer file.Close()

	material, err := s.documents.CreateReportMaterial(r.Context(), s.requestContext(r), service.CreateReportMaterialInput{
		MaterialName: formValue(r, "materialName"),
		MaterialType: formValue(r, "materialType"),
		Category:     formValue(r, "category"),
		Description:  formValue(r, "description"),
		Tags:         formValues(r, "tags"),
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
	writeData(w, r, http.StatusCreated, reportMaterialFromDomain(material))
}

func (s *Server) handleGetReportMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	material, err := s.documents.GetReportMaterial(r.Context(), s.requestContext(r), r.PathValue("materialId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportMaterialFromDomain(material))
}

func (s *Server) handleDeleteReportMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	if err := s.documents.DeleteReportMaterial(r.Context(), s.requestContext(r), r.PathValue("materialId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func reportMaterialsFromDomain(values []service.ReportMaterial) []reportMaterialResponse {
	result := make([]reportMaterialResponse, 0, len(values))
	for _, value := range values {
		result = append(result, reportMaterialFromDomain(value))
	}
	return result
}

func reportMaterialFromDomain(value service.ReportMaterial) reportMaterialResponse {
	tags := append([]string(nil), value.Tags...)
	if tags == nil {
		tags = []string{}
	}
	return reportMaterialResponse{
		ID:           value.ID,
		MaterialName: value.MaterialName,
		MaterialType: value.MaterialType,
		Category:     value.Category,
		Description:  value.Description,
		Tags:         tags,
		Enabled:      value.Enabled,
		Filename:     value.Filename,
		FileSize:     value.FileSize,
		CreatedBy:    value.CreatedBy,
		CreatedAt:    formatTime(value.CreatedAt),
		UpdatedAt:    formatTime(value.UpdatedAt),
	}
}
