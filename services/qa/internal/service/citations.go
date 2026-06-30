package service

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

const (
	maxCitationSnapshotTextRunes    = 2000
	maxCitationSnapshotContextRunes = 4000
)

func citationsFromAgentMessages(messageID, runID string, messages []agent.Message) []Citation {
	citations := make([]Citation, 0)
	seen := map[string]struct{}{}
	for _, message := range messages {
		if message.Role != agent.RoleTool || !isCitationToolName(message.Name) || strings.TrimSpace(message.Content) == "" {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(message.Content), &payload); err != nil {
			continue
		}
		for _, record := range collectCitationRecords(payload) {
			citation, ok := citationFromRecord(record)
			if !ok {
				continue
			}
			citation.ID = newUUID()
			citation.MessageID = messageID
			citation.ResponseRunID = runID
			citation.CitationNo = len(citations) + 1
			citation = NormalizeCitation(citation)
			key := citationSnapshotKey(citation)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			citation.CitationNo = len(citations) + 1
			citations = append(citations, citation)
		}
	}
	return citations
}

func isCitationToolName(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	switch name {
	case "search_knowledge", "get_citation_source", "knowledge_query":
		return true
	}
	return strings.HasSuffix(name, "__search_knowledge") ||
		strings.HasSuffix(name, ".search_knowledge") ||
		strings.HasSuffix(name, "__get_citation_source") ||
		strings.HasSuffix(name, ".get_citation_source")
}

func collectCitationRecords(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, collectCitationRecords(item)...)
		}
		return items
	case map[string]any:
		if looksLikeCitationRecord(typed) {
			return []map[string]any{typed}
		}
		items := []map[string]any{}
		for _, key := range []string{"data", "results", "items", "citations", "references", "documents", "chunks"} {
			if child, ok := typed[key]; ok {
				items = append(items, collectCitationRecords(child)...)
			}
		}
		return items
	default:
		return nil
	}
}

func looksLikeCitationRecord(record map[string]any) bool {
	for _, key := range []string{
		"documentId", "docId", "externalDocId", "external_doc_id",
		"documentName", "docName", "chunkId", "externalChunkId",
		"contentPreview", "content_preview", "quoteText", "quote_text", "text",
	} {
		if _, ok := record[key]; ok {
			return true
		}
	}
	return false
}

func citationFromRecord(record map[string]any) (Citation, bool) {
	citation := Citation{
		DocumentID:              firstString(record, "documentId", "docId", "externalDocId", "external_doc_id"),
		DocumentName:            firstString(record, "documentName", "docName", "document_name", "doc_name", "title", "filename"),
		KnowledgeBaseID:         firstString(record, "knowledgeBaseId", "knowledge_base_id", "externalKbId", "external_kb_id", "kbId"),
		ChunkID:                 firstString(record, "chunkId", "chunk_id", "externalChunkId", "external_chunk_id"),
		SectionPath:             firstString(record, "sectionPath", "section_path"),
		Text:                    firstString(record, "quoteText", "quote_text"),
		ContentPreview:          firstString(record, "contentPreview", "content_preview", "preview"),
		Context:                 firstString(record, "context", "surroundingText", "surrounding_text"),
		ChunkType:               firstString(record, "chunkType", "chunk_type"),
		SourceUnavailableReason: firstString(record, "sourceUnavailableReason", "source_unavailable_reason"),
		Metadata:                firstMap(record, "metadata", "meta"),
	}
	citation.Text = truncateRunes(citation.Text, maxCitationSnapshotTextRunes)
	citation.ContentPreview = truncateRunes(citation.ContentPreview, maxCitationSnapshotTextRunes)
	citation.Context = truncateRunes(citation.Context, maxCitationSnapshotContextRunes)
	if page, ok := firstInt(record, "pageNumber", "page_number", "page"); ok {
		citation.PageNumber = &page
	}
	if score, ok := firstFloat(record, "score", "vectorScore", "vector_score"); ok {
		citation.Score = &score
	}
	if score, ok := firstFloat(record, "rerankScore", "rerank_score"); ok {
		citation.RerankScore = &score
	}
	if available, ok := firstBool(record, "isSourceAvailable", "sourceAvailable", "source_available"); ok {
		citation.IsSourceAvailable = available
	} else {
		citation.IsSourceAvailable = citation.DocumentID != ""
	}
	if firstNonBlank(citation.DocumentID, citation.DocumentName, citation.ChunkID, citation.Text, citation.ContentPreview, citation.Context) == "" {
		return Citation{}, false
	}
	return NormalizeCitation(citation), true
}

func citationSnapshotKey(citation Citation) string {
	return strings.Join([]string{
		citation.KnowledgeBaseID,
		citation.DocumentID,
		citation.ChunkID,
		citation.Text,
		citation.ContentPreview,
	}, "\x00")
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := stringify(record[key]); ok {
			return value
		}
	}
	return ""
}

func firstMap(record map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := record[key].(map[string]any); ok {
			return value
		}
	}
	return map[string]any{}
}

func firstInt(record map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := intValue(record[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func firstFloat(record map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := floatValue(record[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func firstBool(record map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := boolValue(record[key]); ok {
			return value, true
		}
	}
	return false, false
}

func stringify(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case json.Number:
		return typed.String(), typed.String() != ""
	case float64:
		if math.Trunc(typed) == typed {
			return strconv.FormatInt(int64(typed), 10), true
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	default:
		return "", false
	}
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if math.Trunc(typed) == typed {
			return int(typed), true
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func boolValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return false, false
	}
}
