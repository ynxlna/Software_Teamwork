package httpapi

import (
	"net/http"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type reportTypeResponse struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Enabled           bool   `json:"enabled"`
	DefaultTemplateID string `json:"defaultTemplateId,omitempty"`
}

func (s *Server) handleListReportTypes(w http.ResponseWriter, r *http.Request) {
	if !s.requireDocumentService(w, r) {
		return
	}
	types, err := s.documents.ListReportTypes(r.Context(), s.requestContext(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportTypesFromDomain(types))
}

func reportTypesFromDomain(values []service.ReportType) []reportTypeResponse {
	result := make([]reportTypeResponse, 0, len(values))
	for _, value := range values {
		result = append(result, reportTypeResponse{
			Code:              value.Code,
			Name:              value.Name,
			Description:       value.Description,
			Enabled:           value.Enabled,
			DefaultTemplateID: value.DefaultTemplateID,
		})
	}
	return result
}
