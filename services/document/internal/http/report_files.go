package httpapi

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type reportFileDTO struct {
	ID          string `json:"id"`
	ReportID    string `json:"reportId"`
	JobID       string `json:"jobId,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Format      string `json:"format"`
	FileSize    int64  `json:"fileSize,omitempty"`
	Status      string `json:"status"`
	ContentPath string `json:"contentPath,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type createReportFileRequest struct {
	ReportID     string          `json:"reportId"`
	Format       string          `json:"format"`
	TemplateID   string          `json:"templateId,omitempty"`
	StyleOptions json.RawMessage `json:"styleOptions,omitempty"`
}

func (s *Server) requireReportFileService(w http.ResponseWriter, r *http.Request) bool {
	if s.reportFileSvc != nil {
		return true
	}
	writeError(w, r, service.NewError(service.CodeDependency, "report file service is not configured", nil))
	return false
}

func toReportFileDTO(reportFile service.ReportFile) reportFileDTO {
	return reportFileDTO{
		ID:          reportFile.ID,
		ReportID:    reportFile.ReportID,
		JobID:       reportFile.JobID,
		Filename:    reportFile.Filename,
		Format:      reportFile.Format,
		FileSize:    reportFile.FileSize,
		Status:      string(reportFile.Status),
		ContentPath: "/api/v1/report-files/" + reportFile.ID + "/content",
		CreatedBy:   reportFile.CreatedBy,
		CreatedAt:   reportFile.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleListReportFiles(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportFileService(w, r) {
		return
	}
	page, pageSize, err := parsePage(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.reportFileSvc.ListReportFiles(r.Context(), s.requestContext(r), service.ReportFileListFilter{
		Page:     page,
		PageSize: pageSize,
		ReportID: r.URL.Query().Get("reportId"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	data := make([]reportFileDTO, len(result.Items))
	for i, item := range result.Items {
		data[i] = toReportFileDTO(item)
	}
	writePage(w, r, http.StatusOK, data, result.Page)
}

func (s *Server) handleCreateReportFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportFileService(w, r) {
		return
	}
	var body createReportFileRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	reportFile, err := s.reportFileSvc.CreateReportFile(r.Context(), s.requestContext(r), service.CreateReportFileInput{
		ReportID:     body.ReportID,
		Format:       body.Format,
		TemplateID:   body.TemplateID,
		StyleOptions: body.StyleOptions,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusAccepted, toReportFileDTO(reportFile))
}

func (s *Server) handleGetReportFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportFileService(w, r) {
		return
	}
	reportFile, err := s.reportFileSvc.GetReportFile(r.Context(), s.requestContext(r), r.PathValue("reportFileId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, toReportFileDTO(reportFile))
}

func (s *Server) handleGetReportFileContent(w http.ResponseWriter, r *http.Request) {
	if !s.requireReportFileService(w, r) {
		return
	}
	content, err := s.reportFileSvc.ReadReportFileContent(r.Context(), s.requestContext(r), r.PathValue("reportFileId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	defer content.Content.Close()

	contentType := content.ContentType
	if contentType == "" {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	w.Header().Set("Content-Type", contentType)
	if content.Filename != "" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": content.Filename}))
	}
	if content.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(content.SizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, content.Content)
}
