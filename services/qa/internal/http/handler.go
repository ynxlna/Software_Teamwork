package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

type Handler struct {
	conversations *service.ConversationService
}

func NewRouter(conversations *service.ConversationService) http.Handler {
	handler := &Handler{conversations: conversations}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/conversations/", handler.handleConversation)
	mux.HandleFunc("/api/chat/stream", handler.handleChatStream)
	return requestIDMiddleware(mux)
}

func (h *Handler) handleConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, service.NewError(service.CodeValidation, "method not allowed", nil))
		return
	}

	conversationID := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if conversationID == "" || strings.Contains(conversationID, "/") {
		writeError(w, r, service.NewError(service.CodeValidation, "conversation_id is required", nil))
		return
	}

	history, err := h.conversations.GetHistory(r.Context(), authenticatedUserID(r), conversationID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, service.NewError(service.CodeValidation, "method not allowed", nil))
		return
	}

	var request service.StreamRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, r, service.NewError(service.CodeValidation, "invalid stream request", err))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, r, service.NewError(service.CodeValidation, "request body must contain one JSON object", err))
		return
	}

	accepted, err := h.conversations.AcceptCurrentMessage(r.Context(), authenticatedUserID(r), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "event: step\n")
	_ = json.NewEncoder(eventWriter{w: w}).Encode(map[string]any{
		"status":                "accepted",
		"conversation_id":       accepted.ConversationID,
		"context_message_count": accepted.ContextMessageCount,
		"truncated":             accepted.Truncated,
	})
	_, _ = fmt.Fprintf(w, "\n")
}

type eventWriter struct {
	w http.ResponseWriter
}

func (e eventWriter) Write(p []byte) (int, error) {
	_, err := fmt.Fprintf(e.w, "data: %s", p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func authenticatedUserID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-User-ID"))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *service.AppError
	if !errors.As(err, &appErr) {
		appErr = service.NewError(service.CodeInternal, "internal server error", err)
	}

	status := statusForCode(appErr.Code)
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":      appErr.Code,
			"message":   appErr.Message,
			"requestId": requestID(r),
		},
	})
}

func statusForCode(code service.Code) int {
	switch code {
	case service.CodeValidation:
		return http.StatusBadRequest
	case service.CodeUnauthorized:
		return http.StatusUnauthorized
	case service.CodeForbidden:
		return http.StatusForbidden
	case service.CodeNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
