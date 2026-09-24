package service

import (
	"fmt"
	"net/http"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/model"
	"corpus-annotation-agreement-control/backend/internal/repository"
)

func replacementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.CorpusDataset{}, &model.AnnotationSchema{},
		&model.AnnotationSet{}, &model.AdjudicationCase{}, &model.AuditEvent{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	for _, indexName := range []string{"idx_annotation_revision", "idx_annotation_identity"} {
		if err := db.Exec("DROP INDEX IF EXISTS " + indexName).Error; err != nil {
			t.Fatalf("drop legacy index %s: %v", indexName, err)
		}
	}
	if err := db.AutoMigrate(&model.AnnotationSet{}); err != nil {
		t.Fatalf("remigrate annotation sets: %v", err)
	}
	return db
}

func replacementFixture(t *testing.T, db *gorm.DB) (*AnnotationSetService, *AdjudicationCaseService, dto.Actor, dto.Actor, model.CorpusDataset, model.AnnotationSchema) {
	t.Helper()
	users := []model.User{
		{ID: 101, Username: "owner", PasswordHash: "x", Role: constants.RoleAnnotator, Active: true},
		{ID: 102, Username: "peer", PasswordHash: "x", Role: constants.RoleAnnotator, Active: true},
		{ID: 103, Username: "boss", PasswordHash: "x", Role: constants.RoleDataManager, Active: true},
		{ID: 104, Username: "judge", PasswordHash: "x", Role: constants.RoleAdjudicator, Active: true},
		{ID: 105, Username: "second-judge", PasswordHash: "x", Role: constants.RoleAdjudicator, Active: true},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	dataset := model.CorpusDataset{
		ID: 1, DatasetCode: "REP-DS", Name: "Replacement corpus", Language: "en", Domain: "quality",
		ContentMaskPolicy: "Masked", DatasetState: constants.DatasetFrozen, Version: 1, OwnerTeam: "Quality", CreatedBy: 103,
	}
	if err := db.Create(&dataset).Error; err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	schema := model.AnnotationSchema{
		ID: 1, DatasetID: dataset.ID, SchemaCode: "REP-SCHEMA", Version: 1,
		LabelDefinitionsJSON: `[{"code":"RISK","display_name":"Risk","task_type":"classification","description":"risk"},{"code":"CLEAR","display_name":"Clear","task_type":"classification","description":"clear"},{"code":"PARTY","display_name":"Party","task_type":"classification","description":"party"}]`,
		SpanPolicy:           "half-open", OverlapPolicy: "none", ExamplesJSON: "[]",
		SchemaState: constants.SchemaPublished, CreatedBy: 103,
	}
	if err := db.Create(&schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	system := NewSystemService(repository.NewSystemRepository(db), "test-secret-with-at-least-24-bytes", 0)
	annotationRepository := repository.NewAnnotationSetRepository(db)
	caseRepository := repository.NewAdjudicationCaseRepository(db)
	annotationService := NewAnnotationSetService(db, annotationRepository, caseRepository,
		repository.NewCorpusDatasetRepository(db), repository.NewAnnotationSchemaRepository(db), system)
	caseService := NewAdjudicationCaseService(db, caseRepository, annotationRepository, system, "test-algo-v1")
	owner := dto.Actor{ID: 101, Username: "owner", Role: constants.RoleAnnotator}
	peer := dto.Actor{ID: 102, Username: "peer", Role: constants.RoleAnnotator}
	return annotationService, caseService, owner, peer, dataset, schema
}

func createLockedPair(t *testing.T, db *gorm.DB, annotationService *AnnotationSetService, owner, peer dto.Actor, dataset model.CorpusDataset, schema model.AnnotationSchema, itemKey string, leftLabel, rightLabel string) (uint, uint) {
	t.Helper()
	create := func(actor dto.Actor, checksum, label string) uint {
		response, err := annotationService.Create(dto.CreateAnnotationSetRequest{
			DatasetID: dataset.ID, SchemaID: schema.ID, ItemKey: itemKey,
			Labels:         []dto.AnnotationLabel{{UnitKey: "u1", Label: label}},
			SourceChecksum: checksum, QualityNote: "initial",
		}, actor, "req-create")
		if err != nil {
			t.Fatalf("create annotation: %v", err)
		}
		return response.ID
	}
	leftID := create(owner, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", leftLabel)
	rightID := create(peer, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", rightLabel)
	manager := dto.Actor{ID: 103, Username: "boss", Role: constants.RoleDataManager}
	for _, id := range []uint{leftID, rightID} {
		actor := owner
		if id == rightID {
			actor = peer
		}
		if _, err := annotationService.Transition(id, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationSubmitted}, actor, "req-submit"); err != nil {
			t.Fatalf("submit %d: %v", id, err)
		}
		if _, err := annotationService.Transition(id, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationLocked}, manager, "req-lock"); err != nil {
			t.Fatalf("lock %d: %v", id, err)
		}
	}
	return leftID, rightID
}

func assertCaseState(t *testing.T, db *gorm.DB, caseID uint, want string) model.AdjudicationCase {
	t.Helper()
	var adjudication model.AdjudicationCase
	if err := db.First(&adjudication, caseID).Error; err != nil {
		t.Fatalf("load case %d: %v", caseID, err)
	}
	if adjudication.CaseState != want {
		t.Fatalf("case %d state = %s, want %s", caseID, adjudication.CaseState, want)
	}
	return adjudication
}

func TestReplacementDraftRetiresOldResultAndPendingCases(t *testing.T) {
	db := replacementTestDB(t)
	annotationService, caseService, owner, peer, dataset, schema := replacementFixture(t, db)
	leftID, rightID := createLockedPair(t, db, annotationService, owner, peer, dataset, schema, "REP-ITEM-2", "RISK", "CLEAR")
	itemKey := "REP-ITEM-2"

	computeRequest := dto.ComputeAdjudicationRequest{
		DatasetID: dataset.ID, ItemKey: itemKey,
		AnnotationSetIDs: []uint{leftID, rightID}, Metric: "auto",
	}
	firstCase, _, err := caseService.Compute(computeRequest, "compute-1", dto.Actor{ID: 103, Role: constants.RoleDataManager}, "req-compute-1")
	if err != nil {
		t.Fatalf("compute first case: %v", err)
	}
	judge := dto.Actor{ID: 104, Username: "judge", Role: constants.RoleAdjudicator}
	if _, err := caseService.Assign(firstCase.ID, dto.AssignCaseRequest{Note: "Claim before replacement arrives."}, judge, "req-assign"); err != nil {
		t.Fatalf("assign first case: %v", err)
	}
	old, err := annotationService.Get(leftID)
	if err != nil {
		t.Fatalf("load old result: %v", err)
	}
	if !old.CurrentVersion {
		t.Fatal("old result should be current before replacement")
	}

	// Start a replacement from the owner's compared result.
	draft, err := annotationService.Replace(leftID, dto.ReplaceAnnotationSetRequest{Reason: "Boundary was mislabeled and must be corrected."}, owner, "req-replace")
	if err != nil {
		t.Fatalf("start replacement: %v", err)
	}
	if draft.AnnotationState != constants.AnnotationDraft || draft.SupersedesID == nil || *draft.SupersedesID != leftID {
		t.Fatalf("replacement draft is not linked correctly: %+v", draft)
	}
	if draft.ReplacementReason == "" {
		t.Fatal("replacement reason must be persisted")
	}
	// Repeating the replace returns the same draft without overwriting the reason.
	again, err := annotationService.Replace(leftID, dto.ReplaceAnnotationSetRequest{Reason: "different reason ignored"}, owner, "req-replace-again")
	if err != nil {
		t.Fatalf("idempotent replace: %v", err)
	}
	if again.ID != draft.ID || again.ReplacementReason != draft.ReplacementReason {
		t.Fatalf("repeated replace must return the unchanged draft %d, got %d", draft.ID, again.ID)
	}
	// Old result is no longer current and points to its successor.
	retired, err := annotationService.Get(leftID)
	if err != nil {
		t.Fatalf("reload old result: %v", err)
	}
	if retired.CurrentVersion || retired.SupersededByID == nil || *retired.SupersededByID != draft.ID {
		t.Fatalf("old result must point to its successor: %+v", retired)
	}
	// The assigned case becomes pending recomputation and loses its claim.
	pending := assertCaseState(t, db, firstCase.ID, constants.CasePendingRecompute)
	if pending.AdjudicatorID != nil {
		t.Fatal("a pending recomputation case must release its adjudicator claim")
	}

	// A new comparison must not accept the retired old result.
	if _, _, err := caseService.Compute(computeRequest, "compute-stale", dto.Actor{ID: 103, Role: constants.RoleDataManager}, "req-stale"); err == nil {
		t.Fatal("comparison using a replaced old result must be rejected")
	}

	// A non-owner cannot edit the replacement draft; owner corrects and resubmits.
	if _, err := annotationService.Update(draft.ID, dto.UpdateAnnotationSetRequest{
		Labels: []dto.AnnotationLabel{{UnitKey: "u1", Label: "PARTY"}}, QualityNote: "fixed",
	}, peer, "req-hack"); err == nil {
		t.Fatal("non-owner must not edit the replacement draft")
	}
	if _, err := annotationService.Update(draft.ID, dto.UpdateAnnotationSetRequest{
		Labels: []dto.AnnotationLabel{{UnitKey: "u1", Label: "PARTY"}}, QualityNote: "fixed",
	}, owner, "req-update"); err != nil {
		t.Fatalf("owner updates replacement draft: %v", err)
	}
	if _, err := annotationService.Transition(draft.ID, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationSubmitted}, owner, "req-resubmit"); err != nil {
		t.Fatalf("submit replacement draft: %v", err)
	}
	manager := dto.Actor{ID: 103, Role: constants.RoleDataManager}
	if _, err := annotationService.Transition(draft.ID, dto.AnnotationTransitionRequest{TargetState: constants.AnnotationLocked}, manager, "req-relock"); err != nil {
		t.Fatalf("lock replacement draft: %v", err)
	}
	// Locking the revision retires the predecessor as read-only history.
	var predecessor model.AnnotationSet
	if err := db.First(&predecessor, leftID).Error; err != nil {
		t.Fatalf("reload predecessor: %v", err)
	}
	if predecessor.AnnotationState != constants.AnnotationSuperseded {
		t.Fatalf("predecessor state = %s, want superseded", predecessor.AnnotationState)
	}

	// Recompute with the new revision (PARTY still disagrees with peer CLEAR).
	recompute := dto.ComputeAdjudicationRequest{
		DatasetID: dataset.ID, ItemKey: itemKey,
		AnnotationSetIDs: []uint{draft.ID, rightID}, Metric: "auto",
	}
	newCase, _, err := caseService.Compute(recompute, "compute-2", dto.Actor{ID: 103, Role: constants.RoleDataManager}, "req-recompute")
	if err != nil {
		t.Fatalf("recompute with replacement: %v", err)
	}
	if newCase.ID == firstCase.ID {
		t.Fatal("recomputation with the replacement revision must create a new case")
	}
	superseded := assertCaseState(t, db, firstCase.ID, constants.CaseSuperseded)
	if superseded.SupersededByCaseID == nil || *superseded.SupersededByCaseID != newCase.ID {
		t.Fatalf("old case must be linked to the new case, got %+v", superseded.SupersededByCaseID)
	}
	// The new revision is now compared and current; the old one stays history.
	if newRevision, err := annotationService.Get(draft.ID); err != nil || !newRevision.CurrentVersion {
		t.Fatalf("locked replacement must be current after recompute: %v", err)
	}

	// The version chain exposes both versions and the recorded reason.
	chain, err := annotationService.VersionChain(dataset.ID, itemKey, owner.ID)
	if err != nil {
		t.Fatalf("load version chain: %v", err)
	}
	if len(chain.Links) != 2 {
		t.Fatalf("version chain length = %d, want 2", len(chain.Links))
	}
	if chain.Links[0].ID != leftID || chain.Links[1].ID != draft.ID {
		t.Fatalf("version chain order wrong: %+v", chain.Links)
	}
	if chain.Links[1].ReplacementReason == "" || chain.Links[1].SupersedesID == nil {
		t.Fatal("newest chain link must carry the replacement reason and predecessor id")
	}
}

func TestAcceptedCaseStaysReadonlyWhenAnnotationReplaced(t *testing.T) {
	db := replacementTestDB(t)
	annotationService, caseService, owner, peer, dataset, schema := replacementFixture(t, db)
	leftID, rightID := createLockedPair(t, db, annotationService, owner, peer, dataset, schema, "REP-ACCEPTED", "RISK", "CLEAR")
	request := dto.ComputeAdjudicationRequest{
		DatasetID: dataset.ID, ItemKey: "REP-ACCEPTED",
		AnnotationSetIDs: []uint{leftID, rightID}, Metric: "auto",
	}
	created, _, err := caseService.Compute(request, "accepted-compute", dto.Actor{ID: 103, Role: constants.RoleDataManager}, "req-c")
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	judge := dto.Actor{ID: 104, Username: "judge", Role: constants.RoleAdjudicator}
	reviewer := dto.Actor{ID: 105, Username: "second-judge", Role: constants.RoleAdjudicator}
	if _, err := caseService.Assign(created.ID, dto.AssignCaseRequest{Note: "Claiming the open case."}, judge, "req-assign"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, _, err := caseService.Decide(created.ID, dto.AdjudicateCaseRequest{
		FinalLabels: []dto.AnnotationLabel{{UnitKey: "u1", Label: "RISK"}},
		Rationale:   "The published schema supports RISK for this unit.",
	}, "decision-1", judge, "req-decide"); err != nil {
		t.Fatalf("decide: %v", err)
	}
	if _, err := caseService.Review(created.ID, dto.CaseReviewRequest{Note: "Independent review agrees with the decision."}, reviewer, "req-review"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := caseService.Accept(created.ID, dto.CaseReviewRequest{Note: "Accepting after independent review."}, reviewer, "req-accept"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	if _, err := annotationService.Replace(leftID, dto.ReplaceAnnotationSetRequest{Reason: "Correcting after acceptance for traceability."}, owner, "req-replace-accepted"); err != nil {
		t.Fatalf("replace after acceptance: %v", err)
	}
	accepted := assertCaseState(t, db, created.ID, constants.CaseAccepted)
	if accepted.SupersededByCaseID != nil {
		t.Fatal("accepted history must not be linked as superseded by a recomputation")
	}
}

func TestReplaceRequiresOwnerAndReplaceableState(t *testing.T) {
	db := replacementTestDB(t)
	annotationService, _, owner, peer, dataset, schema := replacementFixture(t, db)
	leftID, _ := createLockedPair(t, db, annotationService, owner, peer, dataset, schema, "REP-AUTH", "RISK", "CLEAR")

	other := dto.Actor{ID: 102, Username: "peer", Role: constants.RoleAnnotator}
	if _, err := annotationService.Replace(leftID, dto.ReplaceAnnotationSetRequest{Reason: "Not allowed for peers."}, other, "req-forbidden"); err == nil {
		t.Fatal("a non-owner annotator must not replace someone else's result")
	}

	// A fresh draft cannot be replaced.
	draft, err := annotationService.Create(dto.CreateAnnotationSetRequest{
		DatasetID: dataset.ID, SchemaID: schema.ID, ItemKey: "REP-DRAFT",
		Labels:         []dto.AnnotationLabel{{UnitKey: "u1", Label: "RISK"}},
		SourceChecksum: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		QualityNote:    "draft",
	}, owner, "req-draft")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	_, err = annotationService.Replace(draft.ID, dto.ReplaceAnnotationSetRequest{Reason: "Drafts are not replaceable."}, owner, "req-draft-replace")
	var appErr *AppError
	if err == nil {
		t.Fatal("replacing a draft must fail")
	} else if asApp, ok := err.(*AppError); ok {
		appErr = asApp
	}
	if appErr == nil || appErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %v", err)
	}
}
