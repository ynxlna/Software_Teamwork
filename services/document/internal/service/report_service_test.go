package service

import (
	"context"
	"testing"
	"time"
)

// fakeReportRepository is an in-memory ReportRepository used to unit test
// ReportService business rules without standing up PostgreSQL.
type fakeReportRepository struct {
	reports        map[string]Report
	outlines       map[string]ReportOutline
	sections       map[string]ReportSection
	sectionVersion map[string][]ReportSectionVersion
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{
		reports:        map[string]Report{},
		outlines:       map[string]ReportOutline{},
		sections:       map[string]ReportSection{},
		sectionVersion: map[string][]ReportSectionVersion{},
	}
}

func (f *fakeReportRepository) CreateReport(_ context.Context, value Report) (Report, error) {
	f.reports[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) GetReportByID(_ context.Context, id string) (Report, error) {
	report, ok := f.reports[id]
	if !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	return report, nil
}

func (f *fakeReportRepository) ListReports(_ context.Context, filter ReportListFilter) ([]Report, int, error) {
	var result []Report
	for _, report := range f.reports {
		if filter.CreatorID != "" && report.CreatorID != filter.CreatorID {
			continue
		}
		result = append(result, report)
	}
	return result, len(result), nil
}

func (f *fakeReportRepository) UpdateReport(_ context.Context, value Report) (Report, error) {
	if _, ok := f.reports[value.ID]; !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	f.reports[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) SoftDeleteReport(_ context.Context, id string, deletedAt time.Time) (Report, error) {
	report, ok := f.reports[id]
	if !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	report.Status = ReportStatusDeleted
	report.DeletedAt = &deletedAt
	f.reports[id] = report
	return report, nil
}

func (f *fakeReportRepository) CreateReportOutline(_ context.Context, value ReportOutline) (ReportOutline, error) {
	if value.IsCurrent {
		for id, outline := range f.outlines {
			if outline.ReportID == value.ReportID {
				outline.IsCurrent = false
				f.outlines[id] = outline
			}
		}
	}
	f.outlines[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) ListReportOutlines(_ context.Context, reportID string) ([]ReportOutline, error) {
	var result []ReportOutline
	for _, outline := range f.outlines {
		if outline.ReportID == reportID {
			result = append(result, outline)
		}
	}
	return result, nil
}

func (f *fakeReportRepository) GetReportOutlineByID(_ context.Context, id string) (ReportOutline, error) {
	outline, ok := f.outlines[id]
	if !ok {
		return ReportOutline{}, NewError(CodeNotFound, "report outline not found", nil)
	}
	return outline, nil
}

func (f *fakeReportRepository) UpdateReportOutline(_ context.Context, value ReportOutline) (ReportOutline, error) {
	if _, ok := f.outlines[value.ID]; !ok {
		return ReportOutline{}, NewError(CodeNotFound, "report outline not found", nil)
	}
	f.outlines[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) CreateReportSection(_ context.Context, value ReportSection) (ReportSection, error) {
	f.sections[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) ListReportSections(_ context.Context, reportID string) ([]ReportSection, error) {
	var result []ReportSection
	for _, section := range f.sections {
		if section.ReportID == reportID {
			result = append(result, section)
		}
	}
	return result, nil
}

func (f *fakeReportRepository) GetReportSectionByID(_ context.Context, id string) (ReportSection, error) {
	section, ok := f.sections[id]
	if !ok {
		return ReportSection{}, NewError(CodeNotFound, "report section not found", nil)
	}
	return section, nil
}

func (f *fakeReportRepository) UpdateReportSection(_ context.Context, value ReportSection) (ReportSection, error) {
	if _, ok := f.sections[value.ID]; !ok {
		return ReportSection{}, NewError(CodeNotFound, "report section not found", nil)
	}
	f.sections[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) WithinTx(ctx context.Context, fn func(ReportRepository) error) error {
	return fn(f)
}

func (f *fakeReportRepository) CreateReportSectionVersion(_ context.Context, value ReportSectionVersion) (ReportSectionVersion, error) {
	f.sectionVersion[value.SectionID] = append(f.sectionVersion[value.SectionID], value)
	return value, nil
}

func (f *fakeReportRepository) ListReportSectionVersions(_ context.Context, sectionID string) ([]ReportSectionVersion, error) {
	return f.sectionVersion[sectionID], nil
}

func newTestService() (*ReportService, *fakeReportRepository) {
	repo := newFakeReportRepository()
	svc := NewReportService(repo)
	svc.clock = func() time.Time { return time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC) }
	return svc, repo
}

func mustCreateReport(t *testing.T, svc *ReportService, owner string) Report {
	t.Helper()
	report, err := svc.CreateReport(context.Background(), RequestContext{UserID: owner}, CreateReportInput{
		Name:       "June report",
		ReportType: "summer_peak_inspection",
		TemplateID: "tpl-1",
		Topic:      "summer peak",
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	return report
}

func TestCreateReportValidatesRequiredFields(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.CreateReport(context.Background(), RequestContext{UserID: "u1"}, CreateReportInput{})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestStandardUserCannotAccessOthersReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")

	_, err := svc.GetReport(context.Background(), RequestContext{UserID: "intruder"}, report.ID)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestAdminCanAccessOthersReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")

	got, err := svc.GetReport(context.Background(), RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, report.ID)
	if err != nil {
		t.Fatalf("admin GetReport() error = %v", err)
	}
	if got.ID != report.ID {
		t.Fatalf("got report %q, want %q", got.ID, report.ID)
	}
}

func TestListReportsScopedToOwnerForStandardUser(t *testing.T) {
	svc, _ := newTestService()
	mustCreateReport(t, svc, "owner-1")
	mustCreateReport(t, svc, "owner-2")

	result, err := svc.ListReports(context.Background(), RequestContext{UserID: "owner-1"}, ReportListFilter{})
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if result.Page.Total != 1 || len(result.Items) != 1 || result.Items[0].CreatorID != "owner-1" {
		t.Fatalf("expected only owner-1's report, got %+v", result)
	}
}

func TestSoftDeleteReportIsIdempotentAndConflicts(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	if err := svc.SoftDeleteReport(context.Background(), actor, report.ID); err != nil {
		t.Fatalf("first SoftDeleteReport() error = %v", err)
	}

	err := svc.SoftDeleteReport(context.Background(), actor, report.ID)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict on second delete, got %v", err)
	}
}

func TestUpdateReportRejectsDeletedReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}
	if err := svc.SoftDeleteReport(context.Background(), actor, report.ID); err != nil {
		t.Fatalf("SoftDeleteReport() error = %v", err)
	}

	newTopic := "updated topic"
	_, err := svc.UpdateReport(context.Background(), actor, report.ID, UpdateReportInput{Topic: &newTopic})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict updating deleted report, got %v", err)
	}
}

func TestCreateOutlineRenumbersAndVersions(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source: OutlineSourceManual,
		Sections: []ReportOutlineNode{
			{Title: "Intro"},
			{Title: "Body", Children: []ReportOutlineNode{{Title: "Detail"}}},
		},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}
	if outline.Version != 1 || !outline.IsCurrent {
		t.Fatalf("unexpected outline version/current: %+v", outline)
	}
	if outline.Sections[1].Children[0].Numbering != "2.1" {
		t.Fatalf("expected renumbered child 2.1, got %q", outline.Sections[1].Children[0].Numbering)
	}

	second, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source:   OutlineSourceAI,
		Sections: []ReportOutlineNode{{Title: "Regenerated"}},
	})
	if err != nil {
		t.Fatalf("second CreateOutline() error = %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
}

func TestDeleteOutlineSectionRenumbersRemaining(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source: OutlineSourceManual,
		Sections: []ReportOutlineNode{
			{Title: "Intro"},
			{Title: "Body"},
			{Title: "Conclusion"},
		},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}
	bodyID := outline.Sections[1].ID

	updated, err := svc.DeleteOutlineSection(context.Background(), actor, report.ID, outline.ID, bodyID)
	if err != nil {
		t.Fatalf("DeleteOutlineSection() error = %v", err)
	}
	if len(updated.Sections) != 2 {
		t.Fatalf("expected 2 remaining sections, got %d", len(updated.Sections))
	}
	if updated.Sections[1].Numbering != "2" {
		t.Fatalf("expected conclusion renumbered to 2, got %q", updated.Sections[1].Numbering)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited = true after delete")
	}
}

func TestDeleteOutlineSectionNotFound(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}
	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source:   OutlineSourceManual,
		Sections: []ReportOutlineNode{{Title: "Intro"}},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}

	_, err = svc.DeleteOutlineSection(context.Background(), actor, report.ID, outline.ID, "missing-node")
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeNotFound {
		t.Fatalf("expected not_found error, got %v", err)
	}
}

func TestUpdateSectionMarksManualEditedAndBumpsVersion(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", section.Version)
	}

	newContent := "edited body"
	updated, err := svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	if err != nil {
		t.Fatalf("UpdateSection() error = %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version bumped to 2, got %d", updated.Version)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited = true")
	}
	if updated.ContentSource != ContentSourceManual {
		t.Fatalf("expected contentSource manual, got %q", updated.ContentSource)
	}
}

func TestUpdateSectionContentEditCannotBeUnmarkedAsManual(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	newContent := "edited body"
	manualEdited := false
	updated, err := svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{
		Content:      &newContent,
		ManualEdited: &manualEdited,
	})
	if err != nil {
		t.Fatalf("UpdateSection() error = %v", err)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited to stay true even though the request set manualEdited:false alongside a content change")
	}
}

func TestSaveSectionsUpdatesExistingAndCreatesNewSections(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	existing, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
		Tables:  []map[string]any{{"name": "old"}},
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	newTitle := "Updated intro"
	newContent := "edited body"
	newTables := []map[string]any{{"name": "updated"}}
	createdTitle := "New section"
	createdContent := "new body"
	sections, err := svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{
				ID:      existing.ID,
				Title:   &newTitle,
				Content: &newContent,
				Tables:  &newTables,
			},
			{
				Title:   &createdTitle,
				Content: &createdContent,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("SaveSections() len = %d, want 2", len(sections))
	}

	updated := sections[0]
	if updated.ID != existing.ID {
		t.Fatalf("first section ID = %q, want %q", updated.ID, existing.ID)
	}
	if updated.Title != newTitle || updated.Content != newContent {
		t.Fatalf("updated section did not preserve requested fields: %+v", updated)
	}
	if updated.Version != existing.Version+1 {
		t.Fatalf("updated version = %d, want %d", updated.Version, existing.Version+1)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected updated section to be marked manual edited")
	}

	created := sections[1]
	if created.ID == "" || created.ID == existing.ID {
		t.Fatalf("new section ID was not generated: %+v", created)
	}
	if created.Title != createdTitle || created.Content != createdContent {
		t.Fatalf("created section did not preserve requested fields: %+v", created)
	}
	if created.ManualEdited != true || created.Version != 1 {
		t.Fatalf("unexpected created manual/version fields: %+v", created)
	}
	if repo.sections[existing.ID].Content != newContent {
		t.Fatalf("repository did not persist updated section: %+v", repo.sections[existing.ID])
	}
	if _, ok := repo.sections[created.ID]; !ok {
		t.Fatalf("repository did not persist created section %q", created.ID)
	}
}

func TestSaveSectionsUpdatesMetadataWithoutBumpingVersion(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	existing, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:         "Intro",
		Level:         1,
		Numbering:     "1",
		OutlineNodeID: "outline-1",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	parent, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateSection(parent) error = %v", err)
	}

	parentID := parent.ID
	outlineNodeID := "outline-2"
	title := "Updated intro"
	level := 2
	numbering := "1.1"
	manualEdited := false
	sections, err := svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{{
			ID:            existing.ID,
			ParentID:      &parentID,
			OutlineNodeID: &outlineNodeID,
			Title:         &title,
			Level:         &level,
			Numbering:     &numbering,
			ManualEdited:  &manualEdited,
		}},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("SaveSections() len = %d, want 1", len(sections))
	}

	updated := sections[0]
	if updated.ParentID != parentID || updated.OutlineNodeID != outlineNodeID || updated.Level != level || updated.Numbering != numbering {
		t.Fatalf("metadata fields were not saved: %+v", updated)
	}
	if updated.Title != title {
		t.Fatalf("Title = %q, want %q", updated.Title, title)
	}
	if updated.Version != existing.Version {
		t.Fatalf("metadata-only save bumped version to %d, want %d", updated.Version, existing.Version)
	}
	if updated.ManualEdited {
		t.Fatalf("metadata-only save should respect manualEdited=false when content is unchanged")
	}
}

func TestCreateSectionRejectsParentFromAnotherReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	otherReport := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	otherParent, err := svc.CreateSection(context.Background(), actor, otherReport.ID, CreateSectionInput{Title: "Other parent"})
	if err != nil {
		t.Fatalf("CreateSection(other parent) error = %v", err)
	}

	_, err = svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Child", ParentID: otherParent.ID})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Fields["parentId"] == "" {
		t.Fatalf("expected parentId validation error, got %v", err)
	}
}

func TestSaveSectionsRejectsParentCycle(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	first, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "First"})
	if err != nil {
		t.Fatalf("CreateSection(first) error = %v", err)
	}
	second, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Second"})
	if err != nil {
		t.Fatalf("CreateSection(second) error = %v", err)
	}

	firstParent := second.ID
	secondParent := first.ID
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{ID: first.ID, ParentID: &firstParent},
			{ID: second.ID, ParentID: &secondParent},
		},
	})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Fields["parentId"] == "" {
		t.Fatalf("expected parentId cycle validation error, got %v", err)
	}
}

func TestSaveSectionsPersistsExplicitSortOrder(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	first, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "First"})
	if err != nil {
		t.Fatalf("CreateSection(first) error = %v", err)
	}
	second, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Second"})
	if err != nil {
		t.Fatalf("CreateSection(second) error = %v", err)
	}

	firstSortOrder := 1
	secondSortOrder := 0
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{ID: second.ID, SortOrder: &secondSortOrder},
			{ID: first.ID, SortOrder: &firstSortOrder},
		},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if repo.sections[first.ID].SortOrder != firstSortOrder || repo.sections[second.ID].SortOrder != secondSortOrder {
		t.Fatalf("sortOrder was not persisted: first=%d second=%d", repo.sections[first.ID].SortOrder, repo.sections[second.ID].SortOrder)
	}
}

func TestCreateSectionPersistsExplicitSortOrder(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	sortOrder := 5
	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:     "Sorted section",
		SortOrder: &sortOrder,
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.SortOrder != sortOrder {
		t.Fatalf("SortOrder = %d, want %d", section.SortOrder, sortOrder)
	}
}

func TestCreateSectionWithoutContentDefaultsToManualSource(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.ContentSource != ContentSourceManual {
		t.Fatalf("expected contentSource manual for a content-less section, got %q", section.ContentSource)
	}
	if section.ManualEdited {
		t.Fatalf("expected manualEdited = false for a section created without content")
	}
}

func TestUpdateSectionConflictsWhileGenerationRunning(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	section.GenerationStatus = JobStatusRunning
	repo.sections[section.ID] = section

	newContent := "should not apply"
	_, err = svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict while generation running, got %v", err)
	}
}

func TestCreateSectionVersionDoesNotRequireRegeneration(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	version, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{Source: ContentSourceManual})
	if err != nil {
		t.Fatalf("CreateSectionVersion() error = %v", err)
	}
	if version.Version != 1 || version.Content != "v1" {
		t.Fatalf("unexpected first version: %+v", version)
	}

	second, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{Source: ContentSourceManual})
	if err != nil {
		t.Fatalf("CreateSectionVersion() error = %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
}
