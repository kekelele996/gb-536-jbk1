package repository

import (
	"encoding/json"
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

// ContainingAnnotationSet returns cases whose frozen annotation id list
// references the given annotation. The id list is stored as a JSON array.
func (repository *AdjudicationCaseRepository) ContainingAnnotationSet(annotationID uint) ([]model.AdjudicationCase, error) {
	var cases []model.AdjudicationCase
	pattern := fmt.Sprintf("%%%d%%", annotationID)
	if err := repository.db.Preload("Dataset").
		Where("annotation_set_ids_json LIKE ?", pattern).Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("find cases referencing annotation set: %w", err)
	}
	filtered := make([]model.AdjudicationCase, 0, len(cases))
	for _, adjudication := range cases {
		if jsonContainsID(adjudication.AnnotationSetIDsJSON, annotationID) {
			filtered = append(filtered, adjudication)
		}
	}
	return filtered, nil
}

// MarkPendingRecompute moves still-active cases that reference a replaced
// annotation out of the live queue. Accepted cases and already superseded
// cases are left untouched. Returns the number of affected rows.
func (repository *AdjudicationCaseRepository) MarkPendingRecompute(annotationID uint, activeStates []string) (int64, error) {
	candidates, err := repository.ContainingAnnotationSet(annotationID)
	if err != nil {
		return 0, err
	}
	ids := make([]uint, 0)
	active := map[string]bool{}
	for _, state := range activeStates {
		active[state] = true
	}
	for _, adjudication := range candidates {
		if active[adjudication.CaseState] {
			ids = append(ids, adjudication.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id IN ? AND case_state IN ?", ids, activeStates).
		Updates(map[string]any{"case_state": "pending_recompute", "adjudicator_id": nil})
	if result.Error != nil {
		return 0, fmt.Errorf("mark cases pending recompute: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// SupersedePending marks pending_recompute cases of the same dataset/item as
// superseded by the freshly computed case. It also clears the decision so the
// stale final labels cannot be mistaken for the current outcome, and returns
// the ids of the cases that were superseded.
func (repository *AdjudicationCaseRepository) SupersedePending(datasetID uint, itemKey string, newCaseID uint) ([]uint, error) {
	var ids []uint
	if err := repository.db.Model(&model.AdjudicationCase{}).
		Where("dataset_id = ? AND item_key = ? AND case_state = ?", datasetID, itemKey, "pending_recompute").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("find pending recompute cases: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	result := repository.db.Model(&model.AdjudicationCase{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"case_state": "superseded", "superseded_by_case_id": newCaseID})
	if result.Error != nil {
		return nil, fmt.Errorf("supersede pending recompute cases: %w", result.Error)
	}
	if int(result.RowsAffected) != len(ids) {
		return nil, ErrStateConflict
	}
	return ids, nil
}

func jsonContainsID(encoded string, target uint) bool {
	var ids []uint
	if err := json.Unmarshal([]byte(encoded), &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
