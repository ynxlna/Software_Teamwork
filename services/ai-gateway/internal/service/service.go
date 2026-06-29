package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	DefaultTimeoutMS          = 60000
	MaxDefaultParametersBytes = 8192
)

type Service struct {
	repo             Repository
	encryptor        *CredentialEncryptor
	chatProvider     ChatProvider
	invoker          ModelInvoker
	defaultTimeoutMS int
}

func New(repo Repository, encryptor *CredentialEncryptor, defaultTimeoutMS int, invokers ...ModelInvoker) *Service {
	if defaultTimeoutMS < 1000 {
		defaultTimeoutMS = DefaultTimeoutMS
	}
	var invoker ModelInvoker
	if len(invokers) > 0 {
		invoker = invokers[0]
	}
	return &Service{repo: repo, encryptor: encryptor, invoker: invoker, defaultTimeoutMS: defaultTimeoutMS}
}

func (s *Service) ListModelProfiles(ctx context.Context, filter ListModelProfilesFilter) ([]ModelProfile, error) {
	if filter.Purpose != nil && !validPurpose(*filter.Purpose) {
		return nil, ValidationError(map[string]string{"purpose": "must be chat, embedding, or rerank"})
	}
	items, err := s.repo.ListModelProfiles(ctx, filter)
	if err != nil {
		return nil, DependencyError("model profile store is unavailable", err)
	}
	return items, nil
}

func (s *Service) GetModelProfile(ctx context.Context, id string) (ModelProfile, error) {
	if strings.TrimSpace(id) == "" {
		return ModelProfile{}, ValidationError(map[string]string{"profileId": "is required"})
	}
	profile, err := s.repo.GetModelProfile(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ModelProfile{}, NotFoundError("model profile not found", err)
		}
		return ModelProfile{}, DependencyError("model profile store is unavailable", err)
	}
	return profile, nil
}

func (s *Service) CreateModelProfile(ctx context.Context, req RequestContext, input CreateModelProfileInput) (ModelProfile, error) {
	fields := validateCreateInput(input, s.defaultTimeoutMS)
	if len(fields) > 0 {
		return ModelProfile{}, ValidationError(fields)
	}
	if s.encryptor == nil {
		return ModelProfile{}, DependencyError("credential encryption is not configured", nil)
	}
	now := time.Now().UTC()
	timeoutMS := s.defaultTimeoutMS
	if input.TimeoutMS != nil {
		timeoutMS = *input.TimeoutMS
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	isDefault := false
	if input.IsDefault != nil {
		isDefault = *input.IsDefault
	}
	supportsStreaming := false
	if input.SupportsStreaming != nil {
		supportsStreaming = *input.SupportsStreaming
	}
	if input.Purpose != PurposeChat {
		supportsStreaming = false
	}
	profileID := strings.TrimSpace(input.ID)
	if profileID == "" {
		profileID = newID("mp")
	}
	profile := ModelProfile{
		ID:                profileID,
		Name:              strings.TrimSpace(input.Name),
		Purpose:           input.Purpose,
		Provider:          input.Provider,
		BaseURL:           strings.TrimSpace(input.BaseURL),
		Model:             strings.TrimSpace(input.Model),
		Enabled:           enabled,
		IsDefault:         isDefault,
		TimeoutMS:         timeoutMS,
		APIKeyConfigured:  true,
		SupportsStreaming: supportsStreaming,
		Dimensions:        cloneIntPtr(input.Dimensions),
		TopN:              cloneIntPtr(input.TopN),
		DefaultParameters: normalizeRaw(input.DefaultParameters),
		CreatedByUserID:   strings.TrimSpace(req.UserID),
		UpdatedByUserID:   strings.TrimSpace(req.UserID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	credential, err := s.encryptor.Encrypt(input.APIKey)
	if err != nil {
		return ModelProfile{}, ValidationError(map[string]string{"apiKey": "is invalid"})
	}
	credential.ID = newID("cred")
	credential.ProfileID = profile.ID
	credential.CreatedByUserID = strings.TrimSpace(req.UserID)
	credential.CreatedAt = now
	profile.CredentialID = credential.ID
	revision := revisionFor(profile, RevisionCreated, req, now, []string{"profile", "credential"})
	created, err := s.repo.CreateModelProfile(ctx, profile, credential, revision)
	if err != nil {
		return ModelProfile{}, classifyWriteError(err)
	}
	return created, nil
}

func (s *Service) UpdateModelProfile(ctx context.Context, req RequestContext, input UpdateModelProfileInput) (ModelProfile, error) {
	if strings.TrimSpace(input.ID) == "" {
		return ModelProfile{}, ValidationError(map[string]string{"profileId": "is required"})
	}
	if !hasUpdateField(input) {
		return ModelProfile{}, ValidationError(map[string]string{"body": "must include at least one update field"})
	}
	current, err := s.GetModelProfile(ctx, input.ID)
	if err != nil {
		return ModelProfile{}, err
	}
	updated := current
	changed := []string{}
	if input.Name != nil {
		updated.Name = strings.TrimSpace(*input.Name)
		changed = append(changed, "name")
	}
	if input.Provider != nil {
		updated.Provider = *input.Provider
		changed = append(changed, "provider")
	}
	if input.BaseURL != nil {
		updated.BaseURL = strings.TrimSpace(*input.BaseURL)
		changed = append(changed, "baseUrl")
	}
	if input.Model != nil {
		updated.Model = strings.TrimSpace(*input.Model)
		changed = append(changed, "model")
	}
	if input.Enabled != nil {
		updated.Enabled = *input.Enabled
		changed = append(changed, "enabled")
	}
	if input.IsDefault != nil {
		updated.IsDefault = *input.IsDefault
		changed = append(changed, "isDefault")
	}
	if input.TimeoutMS != nil {
		updated.TimeoutMS = *input.TimeoutMS
		changed = append(changed, "timeoutMs")
	}
	if input.SupportsStreaming != nil {
		updated.SupportsStreaming = *input.SupportsStreaming
		changed = append(changed, "supportsStreaming")
	}
	if input.Dimensions != nil {
		updated.Dimensions = cloneIntPtr(input.Dimensions)
		changed = append(changed, "dimensions")
	}
	if input.TopN != nil {
		updated.TopN = cloneIntPtr(input.TopN)
		changed = append(changed, "topN")
	}
	if input.DefaultParameters != nil {
		updated.DefaultParameters = normalizeRaw(*input.DefaultParameters)
		changed = append(changed, "defaultParameters")
	}
	if updated.Purpose != PurposeChat {
		updated.SupportsStreaming = false
	}
	now := time.Now().UTC()
	updated.UpdatedAt = now
	updated.UpdatedByUserID = strings.TrimSpace(req.UserID)

	fields := validateProfile(updated)
	if len(fields) > 0 {
		return ModelProfile{}, ValidationError(fields)
	}
	var credential *ProviderCredential
	if input.APIKey != nil && strings.TrimSpace(*input.APIKey) == "" {
		return ModelProfile{}, ValidationError(map[string]string{"apiKey": "must not be empty"})
	}
	if input.APIKey != nil {
		if s.encryptor == nil {
			return ModelProfile{}, DependencyError("credential encryption is not configured", nil)
		}
		encrypted, err := s.encryptor.Encrypt(*input.APIKey)
		if err != nil {
			return ModelProfile{}, ValidationError(map[string]string{"apiKey": "is invalid"})
		}
		encrypted.ID = newID("cred")
		encrypted.ProfileID = updated.ID
		encrypted.CreatedByUserID = strings.TrimSpace(req.UserID)
		encrypted.CreatedAt = now
		credential = &encrypted
		updated.CredentialID = encrypted.ID
		updated.APIKeyConfigured = true
		changed = append(changed, "credential")
	}
	revisionType := RevisionUpdated
	if credential != nil {
		revisionType = RevisionCredentialRotated
	}
	revision := revisionFor(updated, revisionType, req, now, changed)
	saved, err := s.repo.UpdateModelProfile(ctx, updated, credential, revision)
	if err != nil {
		return ModelProfile{}, classifyWriteError(err)
	}
	return saved, nil
}

func (s *Service) DeleteModelProfile(ctx context.Context, req RequestContext, id string) error {
	if strings.TrimSpace(id) == "" {
		return ValidationError(map[string]string{"profileId": "is required"})
	}
	now := time.Now().UTC()
	revision := ModelProfileRevision{
		ID:              newID("rev"),
		ProfileID:       strings.TrimSpace(id),
		ChangeType:      RevisionDeleted,
		ChangedByUserID: strings.TrimSpace(req.UserID),
		CallerService:   strings.TrimSpace(req.CallerService),
		RequestID:       strings.TrimSpace(req.RequestID),
		CreatedAt:       now,
	}
	if err := s.repo.SoftDeleteModelProfile(ctx, strings.TrimSpace(id), now, revision); err != nil {
		if errors.Is(err, ErrNotFound) {
			return NotFoundError("model profile not found", err)
		}
		return DependencyError("model profile store is unavailable", err)
	}
	return nil
}

func (s *Service) CheckReady(ctx context.Context) (Readiness, error) {
	checks := []ReadinessCheck{}
	if err := s.repo.CheckReady(ctx); err != nil {
		checks = append(checks, ReadinessCheck{Name: "config_store", Status: "failed", Message: "configuration store is unavailable"})
		return Readiness{Status: "unavailable", Checks: checks}, nil
	}
	checks = append(checks, ReadinessCheck{Name: "config_store", Status: "ok"})
	profiles, err := s.repo.ListModelProfiles(ctx, ListModelProfilesFilter{})
	if err != nil {
		checks = append(checks, ReadinessCheck{Name: "model_profiles", Status: "failed", Message: "model profile query failed"})
		return Readiness{Status: "unavailable", Checks: checks}, nil
	}
	status := "ok"
	for _, purpose := range []Purpose{PurposeChat, PurposeEmbedding, PurposeRerank} {
		name := string(purpose) + "_profile"
		if hasReadyProfile(profiles, purpose) {
			checks = append(checks, ReadinessCheck{Name: name, Status: "ok"})
			continue
		}
		status = "degraded"
		checks = append(checks, ReadinessCheck{Name: name, Status: "missing"})
	}
	return Readiness{Status: status, Checks: checks}, nil
}

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

func classifyWriteError(err error) error {
	if errors.Is(err, ErrConflict) {
		return ConflictError("model profile state conflict", err)
	}
	if errors.Is(err, ErrNotFound) {
		return NotFoundError("model profile not found", err)
	}
	return DependencyError("model profile store is unavailable", err)
}

func validateCreateInput(input CreateModelProfileInput, defaultTimeout int) map[string]string {
	profile := ModelProfile{
		Name:              strings.TrimSpace(input.Name),
		Purpose:           input.Purpose,
		Provider:          input.Provider,
		BaseURL:           strings.TrimSpace(input.BaseURL),
		Model:             strings.TrimSpace(input.Model),
		Enabled:           true,
		TimeoutMS:         defaultTimeout,
		SupportsStreaming: input.SupportsStreaming != nil && *input.SupportsStreaming,
		Dimensions:        input.Dimensions,
		TopN:              input.TopN,
		DefaultParameters: normalizeRaw(input.DefaultParameters),
	}
	if input.Enabled != nil {
		profile.Enabled = *input.Enabled
	}
	if input.TimeoutMS != nil {
		profile.TimeoutMS = *input.TimeoutMS
	}
	fields := validateProfile(profile)
	if strings.TrimSpace(input.APIKey) == "" {
		fields["apiKey"] = "is required"
	}
	return fields
}

func validateProfile(profile ModelProfile) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(profile.Name) == "" {
		fields["name"] = "is required"
	}
	if !validPurpose(profile.Purpose) {
		fields["purpose"] = "must be chat, embedding, or rerank"
	}
	if !validProvider(profile.Provider) {
		fields["provider"] = "must be openai_compatible, siliconflow, or local_compatible"
	}
	if strings.TrimSpace(profile.Model) == "" {
		fields["model"] = "is required"
	}
	if err := validateBaseURL(profile.BaseURL); err != nil {
		fields["baseUrl"] = err.Error()
	}
	if profile.TimeoutMS < 1000 {
		fields["timeoutMs"] = "must be >= 1000"
	}
	if profile.Purpose == PurposeEmbedding && (profile.Dimensions == nil || *profile.Dimensions <= 0) {
		fields["dimensions"] = "is required for embedding profiles"
	}
	if profile.Purpose == PurposeRerank && (profile.TopN == nil || *profile.TopN <= 0) {
		fields["topN"] = "is required for rerank profiles"
	}
	if profile.Purpose != PurposeChat && profile.SupportsStreaming {
		fields["supportsStreaming"] = "is only supported for chat profiles"
	}
	if err := validateSafeJSON(profile.DefaultParameters); err != nil {
		fields["defaultParameters"] = err.Error()
	}
	return fields
}

func validateBaseURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("must use http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("must not contain credentials")
	}
	for key := range parsed.Query() {
		if containsSensitiveToken(key) {
			return fmt.Errorf("must not contain sensitive query parameters")
		}
	}
	return nil
}

func validateSafeJSON(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > MaxDefaultParametersBytes {
		return fmt.Errorf("is too large")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("must be a valid JSON object")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("must be a JSON object")
	}
	return rejectSensitiveKeys(object)
}

func rejectSensitiveKeys(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if containsSensitiveToken(key) {
				return fmt.Errorf("must not contain sensitive keys")
			}
			if err := rejectSensitiveKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectSensitiveKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsSensitiveToken(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(value, "-", "_"))
	denylist := []string{"api_key", "apikey", "authorization", "bearer", "token", "secret", "password", "credential", "connection_string", "database_url", "prompt", "document_text", "provider_response"}
	return slices.ContainsFunc(denylist, func(token string) bool {
		return strings.Contains(normalized, token)
	})
}

func validPurpose(value Purpose) bool {
	return value == PurposeChat || value == PurposeEmbedding || value == PurposeRerank
}

func validProvider(value Provider) bool {
	return value == ProviderOpenAICompatible || value == ProviderSiliconFlow || value == ProviderLocalCompatible
}

func hasUpdateField(input UpdateModelProfileInput) bool {
	return input.Name != nil || input.Provider != nil || input.BaseURL != nil || input.Model != nil || input.APIKey != nil ||
		input.Enabled != nil || input.IsDefault != nil || input.TimeoutMS != nil || input.SupportsStreaming != nil ||
		input.Dimensions != nil || input.TopN != nil || input.DefaultParameters != nil
}

func hasReadyProfile(profiles []ModelProfile, purpose Purpose) bool {
	for _, profile := range profiles {
		if profile.Purpose == purpose && profile.Enabled && profile.APIKeyConfigured && profile.DeletedAt == nil {
			return true
		}
	}
	return false
}

func revisionFor(profile ModelProfile, changeType RevisionChangeType, req RequestContext, now time.Time, fields []string) ModelProfileRevision {
	changed, _ := json.Marshal(fields)
	after, _ := json.Marshal(SafeProfileSnapshot(profile))
	return ModelProfileRevision{
		ID:                newID("rev"),
		ProfileID:         profile.ID,
		ChangeType:        changeType,
		ChangedFieldsJSON: changed,
		AfterSnapshotJSON: after,
		ChangedByUserID:   strings.TrimSpace(req.UserID),
		CallerService:     strings.TrimSpace(req.CallerService),
		RequestID:         strings.TrimSpace(req.RequestID),
		CreatedAt:         now,
	}
}

func SafeProfileSnapshot(profile ModelProfile) map[string]any {
	return map[string]any{
		"id":                profile.ID,
		"name":              profile.Name,
		"purpose":           profile.Purpose,
		"provider":          profile.Provider,
		"baseUrl":           profile.BaseURL,
		"model":             profile.Model,
		"enabled":           profile.Enabled,
		"isDefault":         profile.IsDefault,
		"timeoutMs":         profile.TimeoutMS,
		"apiKeyConfigured":  profile.APIKeyConfigured,
		"supportsStreaming": profile.SupportsStreaming,
		"dimensions":        profile.Dimensions,
		"topN":              profile.TopN,
	}
}

func normalizeRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func newID(prefix string) string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return prefix + "_" + hex.EncodeToString(data[:])
}
