package httpapi

import (
	"encoding/json"
	"net/http"
)

type paginatedEnvelope struct {
	Data      any            `json:"data"`
	Page      map[string]int `json:"page"`
	RequestID string         `json:"requestId"`
}

func writePaginatedJSON(w http.ResponseWriter, status int, data any, page map[string]int, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(paginatedEnvelope{
		Data:      data,
		Page:      page,
		RequestID: requestID,
	})
}
