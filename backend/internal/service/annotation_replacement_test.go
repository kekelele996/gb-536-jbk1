package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/model"
	"corpus-annotation-agreement-control/backend/internal/repository"
)

type replacementFixture struct {
	db           *gorm.DB
	service      *AnnotationSetService
	cases        *repository.AdjudicationCaseRepository
	dataset      model.CorpusDataset
	schema       model.AnnotationSchema
	oldA         model.AnnotationSet
	oldB         model.AnnotationSet
	openCase     model.AdjudicationCase
	acceptedCase model.AdjudicationCase
}

func setupReplacementFixture(t *testing.T) replacementFixture {
	t.Helper()
	db := testServiceDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.AnnotationSchema{}, &model.AnnotationSet{}); err != nil {
		t.Fatalf("migrate replacement schema: %v", err)
	}
	annotationsRepo := repository.NewAnnotationSetRepository(db)
	if err := annotationsRepo.PrepareRevisionIndexes(); err != nil {
		t.Fatalf("prepare revision indexes: %v", err)
	}
	system := NewSystemService(repository.NewSystemRepository(db), "test-secret-with-at-least-24-bytes", 0)
	datasetsRepo := repository.NewCorpusDatasetRepository(db)
	schemasRepo := repository.NewAnnotationSchemaRepository(db)
	casesRepo := repository.NewAdjudicationCaseRepository(db)

	dataset := model.CorpusDataset{
		DatasetCode: "REPLACE-FIX", Name: "Replacement corpus", Language: "en", Domain: "quality",
		ContentMaskPolicy: "Masked", DatasetState: constants.DatasetFrozen, Version: 1, OwnerTeam: "Quality", CreatedBy: 1,
	}
	if err := db.Create(&dataset).Error; err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	definitionsJSON, _ := json.Marshal([]dto.LabelDefinition{
		{Code: "RISK", DisplayName: "Risk", TaskType: "classification"},
		{Code: "CLEAR", DisplayName: "Clear", TaskType: "classification"},
	})
	schema := model.AnnotationSchema{
		DatasetID: dataset.ID, SchemaCode: "REP-SCHEMA", Version: 1,
		LabelDefinitionsJSON: string(definitionsJSON), SchemaState: constants.SchemaPublished, CreatedBy: 1,
	}
	if err := db.Create(&schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	labelsA := `[{"unit_key":"u1","label":"RISK"}]`
	labelsB := `[{"unit_key":"u1","label":"CLEAR"}]`
	oldA := model.AnnotationSet{
		DatasetID: dataset.ID, SchemaID: schema.ID, AnnotatorID: 31, ItemKey: "ITEM-REP",
		LabelsJSON: labelsA, SourceChecksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AnnotationState: constants.AnnotationCompared, QualityNote: "original A",
	}
	oldB := model.AnnotationSet{
		DatasetID: dataset.ID, SchemaID: schema.ID, AnnotatorID: 32, ItemKey: "ITEM-REP",
		LabelsJSON: labelsB, SourceChecksum: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AnnotationState: constants.AnnotationCompared, QualityNote: "original B",
	}
	if err := db.Create(&oldA).Error; err != nil {
		t.Fatalf("create annotation A: %v", err)
	}
	if err := db.Create(&oldB).Error; err != nil {
		t.Fatalf("create annotation B: %v", err)
	}
	idsJSON, _ := json.Marshal([]uint{oldA.ID, oldB.ID})
	openCase := model.AdjudicationCase{
		DatasetID: dataset.ID, ItemKey: "ITEM-REP", AnnotationSetIDsJSON: string(idsJSON),
		AgreementMetric: "cohen_kappa", Applicability: "nominal", DisagreementType: "label",
		ConfusionSnapshotJSON: "[]", EvidenceSnapshotJSON: "[]", ClusterKey: "schema:label:RISK~CLEAR",
		CaseState: constants.CaseOpen, FinalLabelsJSON: "[]", InputHash: "hash-open", AlgorithmVersion: "v1",
		IdempotencyKey: "open-case-key", CreatedBy: 1,
	}
	reviewerID := uint(99)
	acceptedCase := model.AdjudicationCase{
		DatasetID: dataset.ID, ItemKey: "ITEM-REP", AnnotationSetIDsJSON: string(idsJSON),
		AgreementMetric: "cohen_kappa", Applicability: "nominal", DisagreementType: "label",
		ConfusionSnapshotJSON: "[]", EvidenceSnapshotJSON: "[]", ClusterKey: "schema:label:RISK~CLEAR",
		CaseState: constants.CaseAccepted, FinalLabelsJSON: "[]", InputHash: "hash-accepted", AlgorithmVersion: "v1",
		IdempotencyKey: "accepted-case-key", ReviewedBy: &reviewerID, CreatedBy: 1,
	}
	if err := db.Create(&openCase).Error; err != nil {
		t.Fatalf("create open case: %v", err)
	}
	if err := db.Create(&acceptedCase).Error; err != nil {
		t.Fatalf("create accepted case: %v", err)
	}
	service := NewAnnotationSetService(db, annotationsRepo, datasetsRepo, schemasRepo, casesRepo, system)
	return replacementFixture{
		db: db, service: service, cases: casesRepo, dataset: dataset, schema: schema,
		oldA: oldA, oldB: oldB, openCase: openCase, acceptedCase: acceptedCase,
	}
}

func TestReplaceCreatesDraftAndRetiresCases(t *testing.T) {
	fixture := setupReplacementFixture(t)
	actor := dto.Actor{ID: 31, Username: "annotator_a", Role: constants.RoleAnnotator}

	draft, err := fixture.service.Replace(fixture.oldA.ID, dto.ReplaceAnnotationSetRequest{
		Reason: "Boundary was marked one character too wide.",
	}, actor, "request-replace")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if draft.AnnotationState != constants.AnnotationDraft || draft.SupersedesID == nil || *draft.SupersedesID != fixture.oldA.ID {
		t.Fatalf("replacement draft not linked correctly: %+v", draft)
	}
	if draft.AnnotatorID != 31 || draft.ItemKey != "ITEM-REP" || draft.DatasetID != fixture.dataset.ID || draft.SchemaID != fixture.schema.ID {
		t.Fatalf("replacement draft lost annotator/item identity: %+v", draft)
	}
	if draft.ReplaceReason != "Boundary was marked one character too wide." {
		t.Fatalf("replace reason not propagated: %q", draft.ReplaceReason)
	}
	if len(draft.Labels) != 1 || draft.Labels[0].Label != "RISK" {
		t.Fatalf("replacement draft must seed from old labels: %+v", draft.Labels)
	}
	if len(draft.VersionChain) != 2 {
		t.Fatalf("version chain should contain old and new revision, got %d", len(draft.VersionChain))
	}
	if draft.VersionChain[0].ID != fixture.oldA.ID || draft.VersionChain[1].ID != draft.ID {
		t.Fatalf("version chain order is wrong: %+v", draft.VersionChain)
	}
	if draft.VersionChain[0].AnnotationState != constants.AnnotationSuperseded || draft.VersionChain[1].ReplaceReason == "" {
		t.Fatalf("version chain must show superseded old revision and replacement reason: %+v", draft.VersionChain)
	}

	var oldReloaded model.AnnotationSet
	if err := fixture.db.First(&oldReloaded, fixture.oldA.ID).Error; err != nil {
		t.Fatalf("reload old annotation: %v", err)
	}
	if oldReloaded.AnnotationState != constants.AnnotationSuperseded {
		t.Fatalf("old annotation state = %s, want superseded", oldReloaded.AnnotationState)
	}

	openReloaded, err := fixture.cases.Get(fixture.openCase.ID)
	if err != nil {
		t.Fatalf("reload open case: %v", err)
	}
	if openReloaded.CaseState != constants.CasePendingRecompute {
		t.Fatalf("open case state = %s, want pending_recompute", openReloaded.CaseState)
	}
	if openReloaded.AdjudicatorID != nil {
		t.Fatalf("pending recompute case must release its adjudicator assignment")
	}
	if !containsJSONID(openReloaded.SupersededSetIDsJSON, fixture.oldA.ID) {
		t.Fatalf("pending case must record superseded set id, got %q", openReloaded.SupersededSetIDsJSON)
	}

	acceptedReloaded, err := fixture.cases.Get(fixture.acceptedCase.ID)
	if err != nil {
		t.Fatalf("reload accepted case: %v", err)
	}
	if acceptedReloaded.CaseState != constants.CaseAccepted {
		t.Fatalf("accepted history state = %s, must stay accepted and view-only", acceptedReloaded.CaseState)
	}
	if !containsJSONID(acceptedReloaded.SupersededSetIDsJSON, fixture.oldA.ID) {
		t.Fatalf("accepted case must be tagged with stale evidence, got %q", acceptedReloaded.SupersededSetIDsJSON)
	}
}

func TestReplaceIsIdempotentForSameOldResult(t *testing.T) {
	fixture := setupReplacementFixture(t)
	actor := dto.Actor{ID: 31, Username: "annotator_a", Role: constants.RoleAnnotator}
	request := dto.ReplaceAnnotationSetRequest{Reason: "Boundary was marked one character too wide."}

	first, err := fixture.service.Replace(fixture.oldA.ID, request, actor, "request-replace-1")
	if err != nil {
		t.Fatalf("first replace: %v", err)
	}
	second, err := fixture.service.Replace(fixture.oldA.ID, request, actor, "request-replace-2")
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if first.ID != second.ID || !second.Reused {
		t.Fatalf("repeated replacement must return the same draft: first=%d second=%d reused=%v", first.ID, second.ID, second.Reused)
	}
	var draftCount int64
	if err := fixture.db.Model(&model.AnnotationSet{}).Where("supersedes_id = ?", fixture.oldA.ID).Count(&draftCount).Error; err != nil {
		t.Fatalf("count replacement drafts: %v", err)
	}
	if draftCount != 1 {
		t.Fatalf("expected exactly one replacement draft, got %d", draftCount)
	}
}

func TestReplaceRejectsDraftAndForeignOwner(t *testing.T) {
	fixture := setupReplacementFixture(t)
	draftAnnotation := model.AnnotationSet{
		DatasetID: fixture.dataset.ID, SchemaID: fixture.schema.ID, AnnotatorID: 31, ItemKey: "ITEM-DRAFT",
		LabelsJSON: `[{"unit_key":"u1","label":"RISK"}]`, SourceChecksum: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		AnnotationState: constants.AnnotationDraft,
	}
	if err := fixture.db.Create(&draftAnnotation).Error; err != nil {
		t.Fatalf("create draft annotation: %v", err)
	}
	owner := dto.Actor{ID: 31, Role: constants.RoleAnnotator}
	if _, err := fixture.service.Replace(draftAnnotation.ID, dto.ReplaceAnnotationSetRequest{Reason: "Drafts are edited in place."}, owner, "request-bad-state"); err == nil {
		t.Fatal("expected conflict when replacing a draft")
	} else {
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.Status != http.StatusConflict {
			t.Fatalf("expected 409 conflict, got %v", err)
		}
	}
	otherAnnotator := dto.Actor{ID: 32, Role: constants.RoleAnnotator}
	if _, err := fixture.service.Replace(fixture.oldA.ID, dto.ReplaceAnnotationSetRequest{Reason: "Someone else tries to replace this."}, otherAnnotator, "request-bad-owner"); err == nil {
		t.Fatal("expected forbidden when another annotator starts the replacement")
	} else {
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.Status != http.StatusForbidden {
			t.Fatalf("expected 403 forbidden, got %v", err)
		}
	}
}

func TestReplacementDraftCanSucceedOldVersion(t *testing.T) {
	fixture := setupReplacementFixture(t)
	owner := dto.Actor{ID: 31, Username: "annotator_a", Role: constants.RoleAnnotator}
	draft, err := fixture.service.Replace(fixture.oldA.ID, dto.ReplaceAnnotationSetRequest{Reason: "Boundary was marked one character too wide."}, owner, "request-replace")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := fixture.service.Transition(draft.ID, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationSubmitted}, owner, "request-submit"); err != nil {
		t.Fatalf("submit replacement: %v", err)
	}
	manager := dto.Actor{ID: 7, Username: "manager", Role: constants.RoleDataManager}
	locked, err := fixture.service.Transition(draft.ID, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationLocked}, manager, "request-lock")
	if err != nil {
		t.Fatalf("lock replacement: %v", err)
	}
	if locked.AnnotationState != constants.AnnotationLocked || locked.SupersedesID == nil {
		t.Fatalf("replacement draft should be lockable while keeping its lineage: %+v", locked)
	}

	// Superseded results cannot enter a fresh comparison; the locked
	// replacement can participate alongside the peer's current result.
	annotations := []model.AnnotationSet{}
	if err := fixture.db.Where("id IN ?", []uint{fixture.oldA.ID, fixture.oldB.ID}).Find(&annotations).Error; err != nil {
		t.Fatalf("load old annotations: %v", err)
	}
	_, _, _, err = validateComparableAnnotations(annotations, dto.ComputeAdjudicationRequest{
		DatasetID: fixture.dataset.ID, ItemKey: "ITEM-REP", AnnotationSetIDs: []uint{fixture.oldA.ID, fixture.oldB.ID},
	})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "annotation_superseded" {
		t.Fatalf("expected annotation_superseded conflict, got %v", err)
	}
}

func TestPendingRecomputeCaseCannotBeWorked(t *testing.T) {
	fixture := setupReplacementFixture(t)
	owner := dto.Actor{ID: 31, Role: constants.RoleAnnotator}
	if _, err := fixture.service.Replace(fixture.oldA.ID, dto.ReplaceAnnotationSetRequest{Reason: "Boundary was marked one character too wide."}, owner, "request-replace"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	adjudicationService := NewAdjudicationCaseService(fixture.db, fixture.cases, repository.NewAnnotationSetRepository(fixture.db),
		NewSystemService(repository.NewSystemRepository(fixture.db), "test-secret-with-at-least-24-bytes", 0), "v1")
	adjudicator := dto.Actor{ID: 40, Username: "adjudicator", Role: constants.RoleAdjudicator}
	if _, err := adjudicationService.Assign(fixture.openCase.ID, dto.AssignCaseRequest{Note: "Must not claim stale evidence."}, adjudicator, "request-assign"); err == nil {
		t.Fatal("expected conflict assigning a pending_recompute case")
	} else {
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.Code != "case_pending_recompute" {
			t.Fatalf("expected case_pending_recompute, got %v", err)
		}
	}
}

func containsJSONID(encoded string, id uint) bool {
	ids := []uint{}
	if err := json.Unmarshal([]byte(encoded), &ids); err != nil {
		return false
	}
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
