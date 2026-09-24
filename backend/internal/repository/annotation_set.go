package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/model"
)

type AnnotationSetRepository struct{ db *gorm.DB }

func NewAnnotationSetRepository(db *gorm.DB) *AnnotationSetRepository {
	return &AnnotationSetRepository{db: db}
}

func (repository *AnnotationSetRepository) WithDB(db *gorm.DB) *AnnotationSetRepository {
	return &AnnotationSetRepository{db: db}
}

func (repository *AnnotationSetRepository) Create(annotation *model.AnnotationSet) error {
	if err := repository.db.Create(annotation).Error; err != nil {
		return fmt.Errorf("create annotation set: %w", err)
	}
	return nil
}

func (repository *AnnotationSetRepository) Get(id uint) (model.AnnotationSet, error) {
	var annotation model.AnnotationSet
	if err := repository.db.Preload("Dataset").Preload("Schema").Preload("Annotator").First(&annotation, id).Error; err != nil {
		return annotation, fmt.Errorf("get annotation set: %w", err)
	}
	return annotation, nil
}

func (repository *AnnotationSetRepository) ByIDs(ids []uint) ([]model.AnnotationSet, error) {
	var annotations []model.AnnotationSet
	if err := repository.db.Preload("Dataset").Preload("Schema").Preload("Annotator").
		Where("id IN ?", ids).Order("id ASC").Find(&annotations).Error; err != nil {
		return nil, fmt.Errorf("get annotation sets: %w", err)
	}
	return annotations, nil
}

func (repository *AnnotationSetRepository) List(page, pageSize int, datasetID uint, itemKey, state string) ([]model.AnnotationSet, int64, error) {
	query := repository.db.Model(&model.AnnotationSet{})
	if datasetID > 0 {
		query = query.Where("dataset_id = ?", datasetID)
	}
	if itemKey != "" {
		query = query.Where("item_key = ?", itemKey)
	}
	if state != "" {
		query = query.Where("annotation_state = ?", state)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count annotation sets: %w", err)
	}
	var annotations []model.AnnotationSet
	if err := query.Preload("Dataset").Preload("Schema").Preload("Annotator").
		Order("item_key ASC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&annotations).Error; err != nil {
		return nil, 0, fmt.Errorf("list annotation sets: %w", err)
	}
	return annotations, total, nil
}

func (repository *AnnotationSetRepository) UpdateDraft(id, annotatorID uint, expectedUpdatedAt time.Time, labelsJSON, qualityNote string) error {
	result := repository.db.Model(&model.AnnotationSet{}).
		Where("id = ? AND annotator_id = ? AND annotation_state = ? AND updated_at = ?", id, annotatorID, "draft", expectedUpdatedAt).
		Updates(map[string]any{"labels_json": labelsJSON, "quality_note": qualityNote})
	if result.Error != nil {
		return fmt.Errorf("update annotation draft: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *AnnotationSetRepository) Transition(id uint, from, to string) error {
	updates := map[string]any{"annotation_state": to}
	if to == "submitted" {
		updates["submitted_at"] = time.Now().UTC()
	}
	result := repository.db.Model(&model.AnnotationSet{}).Where("id = ? AND annotation_state = ?", id, from).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("transition annotation set: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}
