package service

import (
	"errors"
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

func TestDatasetCreateRollsBackWhenAuditWriteFails(t *testing.T) {
	db := testServiceDB(t)
	failAuditWrites(t, db)
	system := NewSystemService(repository.NewSystemRepository(db), "test-secret-with-at-least-24-bytes", 0)
	service := NewCorpusDatasetService(db, repository.NewCorpusDatasetRepository(db), system)

	_, err := service.Create(dto.CreateCorpusDatasetRequest{
		DatasetCode: "TX-ROLLBACK", Name: "Transactional corpus", Language: "en",
		Domain: "quality", DocumentCount: 3, ContentMaskPolicy: "Mask source identifiers before review.", OwnerTeam: "Quality",
	}, dto.Actor{ID: 7, Username: "manager", Role: constants.RoleDataManager}, "request-rollback")
	if err == nil {
		t.Fatal("expected the injected audit failure")
	}
	var count int64
	if countErr := db.Model(&model.CorpusDataset{}).Where("dataset_code = ?", "TX-ROLLBACK").Count(&count).Error; countErr != nil {
		t.Fatalf("count datasets: %v", countErr)
	}
	if count != 0 {
		t.Fatalf("business row survived a failed audit write: count=%d", count)
	}
}

func TestCaseTransitionRollsBackWhenAuditWriteFails(t *testing.T) {
	db := testServiceDB(t)
	dataset := model.CorpusDataset{
		DatasetCode: "CASE-TX", Name: "Case corpus", Language: "en", Domain: "quality",
		ContentMaskPolicy: "Masked", DatasetState: constants.DatasetFrozen, Version: 1, OwnerTeam: "Quality", CreatedBy: 1,
	}
	if err := db.Create(&dataset).Error; err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	reviewerID := uint(22)
	caseRecord := model.AdjudicationCase{
		DatasetID: dataset.ID, ItemKey: "ITEM-1", AnnotationSetIDsJSON: "[1,2]",
		AgreementMetric: "cohen_kappa", Applicability: "nominal", DisagreementType: "label",
		ConfusionSnapshotJSON: "[]", EvidenceSnapshotJSON: "[]", ClusterKey: "schema:label:a~b",
		CaseState: constants.CaseReviewed, FinalLabelsJSON: "[]", InputHash: "hash", AlgorithmVersion: "v1",
		IdempotencyKey: "case-tx-key", ReviewedBy: &reviewerID, CreatedBy: 1,
	}
	if err := db.Create(&caseRecord).Error; err != nil {
		t.Fatalf("create case: %v", err)
	}
	failAuditWrites(t, db)
	system := NewSystemService(repository.NewSystemRepository(db), "test-secret-with-at-least-24-bytes", 0)
	target := NewAdjudicationCaseService(db, repository.NewAdjudicationCaseRepository(db), repository.NewAnnotationSetRepository(db), system, "v1")

	_, err := target.caseTransition(caseRecord, constants.CaseAccepted, "Independent review accepted.", dto.Actor{
		ID: reviewerID, Username: "reviewer", Role: constants.RoleAdjudicator,
	}, "request-case-rollback")
	if err == nil {
		t.Fatal("expected the injected audit failure")
	}
	var reloaded model.AdjudicationCase
	if loadErr := db.First(&reloaded, caseRecord.ID).Error; loadErr != nil {
		t.Fatalf("reload case: %v", loadErr)
	}
	if reloaded.CaseState != constants.CaseReviewed {
		t.Fatalf("case transition survived a failed audit write: state=%s", reloaded.CaseState)
	}
}

func TestAcceptRequiresRecordedIndependentReviewer(t *testing.T) {
	db := testServiceDB(t)
	dataset := model.CorpusDataset{
		DatasetCode: "REVIEW-AUTH", Name: "Review corpus", Language: "en", Domain: "quality",
		ContentMaskPolicy: "Masked", DatasetState: constants.DatasetFrozen, Version: 1, OwnerTeam: "Quality", CreatedBy: 1,
	}
	if err := db.Create(&dataset).Error; err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	reviewerID := uint(22)
	caseRecord := model.AdjudicationCase{
		DatasetID: dataset.ID, ItemKey: "ITEM-2", AnnotationSetIDsJSON: "[1,2]",
		AgreementMetric: "cohen_kappa", Applicability: "nominal", DisagreementType: "label",
		ConfusionSnapshotJSON: "[]", EvidenceSnapshotJSON: "[]", ClusterKey: "schema:label:a~b",
		CaseState: constants.CaseReviewed, FinalLabelsJSON: "[]", InputHash: "hash-2", AlgorithmVersion: "v1",
		IdempotencyKey: "review-auth-key", ReviewedBy: &reviewerID, CreatedBy: 1,
	}
	if err := db.Create(&caseRecord).Error; err != nil {
		t.Fatalf("create case: %v", err)
	}
	system := NewSystemService(repository.NewSystemRepository(db), "test-secret-with-at-least-24-bytes", 0)
	target := NewAdjudicationCaseService(db, repository.NewAdjudicationCaseRepository(db), repository.NewAnnotationSetRepository(db), system, "v1")

	_, err := target.Accept(caseRecord.ID, dto.CaseReviewRequest{Note: "Attempt by another adjudicator."}, dto.Actor{
		ID: 23, Username: "other", Role: constants.RoleAdjudicator,
	}, "request-review-auth")
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusForbidden {
		t.Fatalf("expected forbidden for a non-reviewer, got %v", err)
	}
}

func TestAnnotationTransitionAuthorizationIsEnforcedByService(t *testing.T) {
	tests := []struct {
		name       string
		annotation model.AnnotationSet
		target     string
		actor      dto.Actor
		forbidden  bool
	}{
		{
			name: "owner may submit", annotation: model.AnnotationSet{AnnotatorID: 10}, target: constants.AnnotationSubmitted,
			actor: dto.Actor{ID: 10, Role: constants.RoleAnnotator},
		},
		{
			name: "other annotator may not submit", annotation: model.AnnotationSet{AnnotatorID: 10}, target: constants.AnnotationSubmitted,
			actor: dto.Actor{ID: 11, Role: constants.RoleAnnotator}, forbidden: true,
		},
		{
			name: "annotator may not lock", annotation: model.AnnotationSet{AnnotatorID: 10}, target: constants.AnnotationLocked,
			actor: dto.Actor{ID: 10, Role: constants.RoleAnnotator}, forbidden: true,
		},
		{
			name: "manager may lock", annotation: model.AnnotationSet{AnnotatorID: 10}, target: constants.AnnotationLocked,
			actor: dto.Actor{ID: 12, Role: constants.RoleDataManager},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := authorizeAnnotationTransition(test.annotation, test.target, test.actor)
			var appErr *AppError
			isForbidden := errors.As(err, &appErr) && appErr.Status == http.StatusForbidden
			if isForbidden != test.forbidden {
				t.Fatalf("forbidden=%v, error=%v", isForbidden, err)
			}
		})
	}
}

func testServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.CorpusDataset{}, &model.AdjudicationCase{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

func failAuditWrites(t *testing.T, db *gorm.DB) {
	t.Helper()
	callbackName := "test:fail-audit-" + t.Name()
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AuditEvent" {
			tx.AddError(errors.New("injected audit storage failure"))
		}
	}); err != nil {
		t.Fatalf("register audit failure callback: %v", err)
	}
}
