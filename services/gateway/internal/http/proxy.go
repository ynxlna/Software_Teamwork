package httpapi

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/response"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/service"
)

var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func (s *Server) handleProxy(route routeSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authContext, _, ok := s.authenticateRequest(w, r)
		if !ok {
			return
		}

		baseURL := s.ownerBaseURLs[route.Owner]
		if baseURL == nil {
			s.writeDependencyError(w, r, route.Owner+" service is not configured")
			return
		}

		targetURL := *baseURL
		targetURL.Path = joinProxyPath(baseURL.Path, route.downstreamPath(r))
		targetURL.RawQuery = r.URL.RawQuery

		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
		if err != nil {
			s.writeDependencyError(w, r, "downstream request could not be created")
			return
		}
		proxyReq.Header = cloneProxyHeaders(r.Header)
		applyGatewayHeaders(proxyReq, r, authContext, s.internalServiceToken)

		res, err := s.httpClient.Do(proxyReq)
		if err != nil {
			s.logger.WarnContext(r.Context(), "downstream request failed",
				"service", "gateway",
				"request_id", middleware.RequestIDFromContext(r.Context()),
				"operation", route.OperationID,
				"dependency", route.Owner,
				"status", "failed",
			)
			s.writeDependencyError(w, r, route.Owner+" service is unavailable")
			return
		}
		defer res.Body.Close()

		if res.StatusCode >= http.StatusBadRequest {
			s.writeDownstreamError(w, r, route, res)
			return
		}

		copyProxyHeaders(w.Header(), res.Header)
		w.Header().Set("X-Request-Id", middleware.RequestIDFromContext(r.Context()))
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}
}

func (s *Server) writeDownstreamError(w http.ResponseWriter, r *http.Request, route routeSpec, res *http.Response) {
	if res.StatusCode >= http.StatusInternalServerError {
		io.Copy(io.Discard, res.Body)
		s.writeDependencyError(w, r, route.Owner+" service is unavailable")
		return
	}

	requestID := middleware.RequestIDFromContext(r.Context())
	detail := response.ErrorDetail{
		Code:      downstreamErrorCode(res.StatusCode),
		Message:   http.StatusText(res.StatusCode),
		RequestID: requestID,
	}

	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&envelope); err == nil {
		if isPublicErrorCode(envelope.Error.Code) {
			detail.Code = envelope.Error.Code
		}
		if message := strings.TrimSpace(envelope.Error.Message); message != "" {
			detail.Message = message
		}
		detail.Fields = envelope.Error.Fields
	} else {
		io.Copy(io.Discard, res.Body)
	}

	response.WriteError(w, res.StatusCode, detail)
}

func (route routeSpec) downstreamPath(r *http.Request) string {
	if strings.TrimSpace(route.DownstreamPattern) == "" {
		return route.defaultDownstreamPath(r.URL.Path)
	}
	return renderPathTemplate(route.DownstreamPattern, r)
}

func (route routeSpec) defaultDownstreamPath(path string) string {
	suffix, ok := publicAPISuffix(path)
	if !ok {
		return path
	}
	switch route.Owner {
	case "knowledge", "qa":
		return "/internal/v1" + suffix
	case "document":
		return suffix
	default:
		return path
	}
}

func publicAPISuffix(path string) (string, bool) {
	normalized := "/" + strings.TrimLeft(path, "/")
	switch {
	case normalized == "/api/v1":
		return "/", true
	case strings.HasPrefix(normalized, "/api/v1/"):
		return strings.TrimPrefix(normalized, "/api/v1"), true
	default:
		return normalized, false
	}
}

func renderPathTemplate(template string, r *http.Request) string {
	segments := strings.Split(template, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		segments[i] = url.PathEscape(r.PathValue(name))
	}
	return strings.Join(segments, "/")
}

func cloneProxyHeaders(source http.Header) http.Header {
	target := make(http.Header, len(source))
	for key, values := range source {
		if _, skip := hopByHopHeaders[http.CanonicalHeaderKey(key)]; skip {
			continue
		}
		switch http.CanonicalHeaderKey(key) {
		case "Authorization",
			"Forwarded",
			"X-Forwarded-For",
			"X-Forwarded-Host",
			"X-Forwarded-Proto",
			"X-User-Id",
			"X-User-Roles",
			"X-User-Permissions",
			"X-Service-Token",
			"X-Caller-Service":
			continue
		}
		target[key] = append([]string(nil), values...)
	}
	return target
}

func copyProxyHeaders(target http.Header, source http.Header) {
	for key, values := range source {
		if _, skip := hopByHopHeaders[http.CanonicalHeaderKey(key)]; skip {
			continue
		}
		target.Del(key)
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func applyGatewayHeaders(proxyReq *http.Request, incoming *http.Request, authContext service.SessionCacheEntry, serviceToken string) {
	requestID := middleware.RequestIDFromContext(incoming.Context())
	proxyReq.Header.Set("X-Request-Id", requestID)
	proxyReq.Header.Set("X-Caller-Service", "gateway")
	if strings.TrimSpace(serviceToken) != "" {
		proxyReq.Header.Set("X-Service-Token", strings.TrimSpace(serviceToken))
	}
	proxyReq.Header.Set("X-User-Id", authContext.UserID)
	proxyReq.Header.Set("X-User-Roles", strings.Join(authContext.Roles, ","))
	proxyReq.Header.Set("X-User-Permissions", strings.Join(authContext.Permissions, ","))
	proxyReq.Header.Set("X-Forwarded-For", clientIP(incoming))
	proto := "http"
	if incoming.TLS != nil {
		proto = "https"
	}
	proxyReq.Header.Set("X-Forwarded-Proto", proto)
}

func downstreamErrorCode(status int) response.Code {
	switch status {
	case http.StatusUnauthorized:
		return response.CodeUnauthorized
	case http.StatusForbidden:
		return response.CodeForbidden
	case http.StatusNotFound:
		return response.CodeNotFound
	case http.StatusConflict:
		return response.CodeConflict
	case http.StatusTooManyRequests:
		return response.CodeRateLimited
	default:
		return response.CodeValidation
	}
}

func isPublicErrorCode(code response.Code) bool {
	switch code {
	case response.CodeValidation,
		response.CodeUnauthorized,
		response.CodeForbidden,
		response.CodeNotFound,
		response.CodeConflict,
		response.CodeRateLimited:
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func joinProxyPath(base string, path string) string {
	base = strings.TrimRight(base, "/")
	path = "/" + strings.TrimLeft(path, "/")
	if base == "" {
		return path
	}
	return base + path
}
