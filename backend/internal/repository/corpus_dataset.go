package repository

import (
	"fmt"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/model"
)

type CorpusDatasetRepository struct{ db *gorm.DB }

func NewCorpusDatasetRepository(db *gorm.DB) *CorpusDatasetRepository {
	return &CorpusDatasetRepository{db: db}
}

func (repository *CorpusDatasetRepository) WithDB(db *gorm.DB) *CorpusDatasetRepository {
	return &CorpusDatasetRepository{db: db}
}

func (repository *CorpusDatasetRepository) Create(dataset *model.CorpusDataset) error {
	if err := repository.db.Create(dataset).Error; err != nil {
		return fmt.Errorf("create corpus dataset: %w", err)
	}
	return nil
}

func (repository *CorpusDatasetRepository) Get(id uint) (model.CorpusDataset, error) {
	var dataset model.CorpusDataset
	if err := repository.db.First(&dataset, id).Error; err != nil {
		return dataset, fmt.Errorf("get corpus dataset: %w", err)
	}
	return dataset, nil
}

func (repository *CorpusDatasetRepository) List(page, pageSize int, state, language, owner string) ([]model.CorpusDataset, int64, error) {
	query := repository.db.Model(&model.CorpusDataset{})
	if state != "" {
		query = query.Where("dataset_state = ?", state)
	}
	if language != "" {
		query = query.Where("language = ?", language)
	}
	if owner != "" {
		query = query.Where("owner_team = ?", owner)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count corpus datasets: %w", err)
	}
	var datasets []model.CorpusDataset
	if err := query.Order("dataset_code ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&datasets).Error; err != nil {
		return nil, 0, fmt.Errorf("list corpus datasets: %w", err)
	}
	return datasets, total, nil
}

func (repository *CorpusDatasetRepository) Update(dataset *model.CorpusDataset, expectedVersion int) error {
	result := repository.db.Model(&model.CorpusDataset{}).
		Where("id = ? AND version = ? AND dataset_state = ?", dataset.ID, expectedVersion, "draft").
		Updates(map[string]any{
			"name": dataset.Name, "language": dataset.Language, "domain": dataset.Domain,
			"document_count": dataset.DocumentCount, "content_mask_policy": dataset.ContentMaskPolicy,
			"owner_team": dataset.OwnerTeam, "version": expectedVersion + 1,
		})
	if result.Error != nil {
		return fmt.Errorf("update corpus dataset: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (repository *CorpusDatasetRepository) Transition(id uint, expectedVersion int, from, to string) error {
	result := repository.db.Model(&model.CorpusDataset{}).
		Where("id = ? AND version = ? AND dataset_state = ?", id, expectedVersion, from).
		Updates(map[string]any{"dataset_state": to, "version": expectedVersion + 1})
	if result.Error != nil {
		return fmt.Errorf("transition corpus dataset: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *CorpusDatasetRepository) Counts(id uint) (int64, int64, error) {
	var schemas, annotations int64
	if err := repository.db.Model(&model.AnnotationSchema{}).Where("dataset_id = ?", id).Count(&schemas).Error; err != nil {
		return 0, 0, fmt.Errorf("count annotation schemas: %w", err)
	}
	if err := repository.db.Model(&model.AnnotationSet{}).Where("dataset_id = ?", id).Count(&annotations).Error; err != nil {
		return 0, 0, fmt.Errorf("count annotation sets: %w", err)
	}
	return schemas, annotations, nil
}
