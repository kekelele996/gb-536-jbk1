package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/model"
)

type AdjudicationCaseRepository struct{ db *gorm.DB }

func NewAdjudicationCaseRepository(db *gorm.DB) *AdjudicationCaseRepository {
	return &AdjudicationCaseRepository{db: db}
}

func (repository *AdjudicationCaseRepository) WithDB(db *gorm.DB) *AdjudicationCaseRepository {
	return &AdjudicationCaseRepository{db: db}
}

func (repository *AdjudicationCaseRepository) Create(adjudication *model.AdjudicationCase) error {
	if err := repository.db.Create(adjudication).Error; err != nil {
		return fmt.Errorf("create adjudication case: %w", err)
	}
	return nil
}

func (repository *AdjudicationCaseRepository) Get(id uint) (model.AdjudicationCase, error) {
	var adjudication model.AdjudicationCase
	if err := repository.db.Preload("Dataset").First(&adjudication, id).Error; err != nil {
		return adjudication, fmt.Errorf("get adjudication case: %w", err)
	}
	return adjudication, nil
}

func (repository *AdjudicationCaseRepository) List(page, pageSize int, datasetID uint, state, disagreementType, clusterKey string) ([]model.AdjudicationCase, int64, error) {
	query := repository.db.Model(&model.AdjudicationCase{})
	if datasetID > 0 {
		query = query.Where("dataset_id = ?", datasetID)
	}
	if state != "" {
		query = query.Where("case_state = ?", state)
	}
	if disagreementType != "" {
		query = query.Where("disagreement_type = ?", disagreementType)
	}
	if clusterKey != "" {
		query = query.Where("cluster_key = ?", clusterKey)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count adjudication cases: %w", err)
	}
	var cases []model.AdjudicationCase
	if err := query.Preload("Dataset").Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&cases).Error; err != nil {
		return nil, 0, fmt.Errorf("list adjudication cases: %w", err)
	}
	return cases, total, nil
}

func (repository *AdjudicationCaseRepository) FindByIdempotencyKey(key string) (model.AdjudicationCase, error) {
	var adjudication model.AdjudicationCase
	if err := repository.db.Preload("Dataset").Where("idempotency_key = ?", key).First(&adjudication).Error; err != nil {
		return adjudication, fmt.Errorf("find idempotent adjudication: %w", err)
	}
	return adjudication, nil
}

func (repository *AdjudicationCaseRepository) LatestByInput(inputHash, algorithmVersion string) (model.AdjudicationCase, error) {
	var adjudication model.AdjudicationCase
	if err := repository.db.Preload("Dataset").
		Where("input_hash = ? AND algorithm_version = ?", inputHash, algorithmVersion).
		Order("id DESC").First(&adjudication).Error; err != nil {
		return adjudication, fmt.Errorf("find computed adjudication: %w", err)
	}
	return adjudication, nil
}

func (repository *AdjudicationCaseRepository) Assign(id uint, actorID uint) error {
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id = ? AND case_state IN ?", id, []string{"open", "reopened"}).
		Updates(map[string]any{"case_state": "assigned", "adjudicator_id": actorID})
	if result.Error != nil {
		return fmt.Errorf("assign adjudication case: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *AdjudicationCaseRepository) Decide(id, actorID uint, labelsJSON, rationale, idempotencyKey string) error {
	now := time.Now().UTC()
	key := idempotencyKey
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id = ? AND case_state = ? AND adjudicator_id = ?", id, "assigned", actorID).
		Updates(map[string]any{
			"case_state": "adjudicated", "final_labels_json": labelsJSON,
			"rationale": rationale, "decided_at": now, "decision_idempotency_key": &key,
		})
	if result.Error != nil {
		return fmt.Errorf("decide adjudication case: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *AdjudicationCaseRepository) Review(id uint, from, to string, reviewerID uint, rationale string) error {
	updates := map[string]any{"case_state": to, "reviewed_by": reviewerID}
	if rationale != "" {
		updates["rationale"] = gorm.Expr("rationale || ?", "\nReview: "+rationale)
	}
	if to == "reopened" {
		updates["reopen_count"] = gorm.Expr("reopen_count + 1")
		updates["adjudicator_id"] = nil
	}
	result := repository.db.Model(&model.AdjudicationCase{}).Where("id = ? AND case_state = ?", id, from).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("review adjudication case: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

// CasesForItem returns every adjudication case for a dataset item, ordered
// oldest first. Callers decode the frozen annotation id snapshot to decide
// which cases reference a replaced annotation set.
func (repository *AdjudicationCaseRepository) CasesForItem(datasetID uint, itemKey string) ([]model.AdjudicationCase, error) {
	var cases []model.AdjudicationCase
	if err := repository.db.Preload("Dataset").
		Where("dataset_id = ? AND item_key = ?", datasetID, itemKey).
		Order("id ASC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list adjudication cases for item: %w", err)
	}
	return cases, nil
}

// MarkPendingRecompute moves one still-active case to pending_recompute with a
// conditional update and records the stale annotation set id on the case.
func (repository *AdjudicationCaseRepository) MarkPendingRecompute(id uint, supersededIDsJSON string) error {
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id = ? AND case_state IN ?", id, []string{"open", "assigned", "adjudicated", "reviewed", "reopened"}).
		Updates(map[string]any{
			"case_state":              "pending_recompute",
			"superseded_set_ids_json": supersededIDsJSON,
			"adjudicator_id":          nil,
		})
	if result.Error != nil {
		return fmt.Errorf("mark case pending recompute: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

// RecordStaleAcceptedHistory appends the replaced annotation set id to an
// accepted case without changing its state, so the frozen decision stays
// view-only but visibly tied to a superseded evidence version.
func (repository *AdjudicationCaseRepository) RecordStaleAcceptedHistory(id uint, supersededIDsJSON string) error {
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id = ? AND case_state = ?", id, "accepted").
		Update("superseded_set_ids_json", supersededIDsJSON)
	if result.Error != nil {
		return fmt.Errorf("record stale accepted history: %w", result.Error)
	}
	if result.RowsAffected > 1 {
		return fmt.Errorf("record stale accepted history: %d rows updated", result.RowsAffected)
	}
	return nil
}
