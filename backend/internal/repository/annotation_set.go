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

// PrepareRevisionIndexes removes the early combined unique index that included
// source_checksum: a replacement draft legitimately reuses the same source
// digest. Current revisions are protected by a partial unique index created in
// EnsureRevisionIndexes.
func (repository *AnnotationSetRepository) PrepareRevisionIndexes() error {
	migrator := repository.db.Migrator()
	if migrator.HasIndex(&model.AnnotationSet{}, "idx_annotation_revision") {
		if err := migrator.DropIndex(&model.AnnotationSet{}, "idx_annotation_revision"); err != nil {
			return fmt.Errorf("drop legacy annotation revision index: %w", err)
		}
	}
	return EnsureRevisionIndexes(repository.db)
}

// EnsureRevisionIndexes enforces at most one live revision per annotator and
// item. Superseded rows are exempt, so the version history can share the same
// (dataset, schema, annotator, item) tuple. Partial unique indexes are
// supported by both PostgreSQL and SQLite.
func EnsureRevisionIndexes(db *gorm.DB) error {
	statement := "CREATE UNIQUE INDEX IF NOT EXISTS idx_annotation_current_revision ON annotation_sets (dataset_id, schema_id, annotator_id, item_key) WHERE annotation_state <> 'superseded'"
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("create current revision index: %w", err)
	}
	return nil
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

// FindReplacement returns the draft (or later revision) created to replace the
// given old result. The unique index on supersedes_id guarantees at most one.
func (repository *AnnotationSetRepository) FindReplacement(supersedesID uint) (model.AnnotationSet, error) {
	var annotation model.AnnotationSet
	if err := repository.db.Preload("Dataset").Preload("Schema").Preload("Annotator").
		Where("supersedes_id = ?", supersedesID).First(&annotation).Error; err != nil {
		return annotation, fmt.Errorf("find replacement annotation set: %w", err)
	}
	return annotation, nil
}

// VersionChain loads every revision sharing the annotation's dataset, schema,
// annotator and item, oldest first.
func (repository *AnnotationSetRepository) VersionChain(annotation model.AnnotationSet) ([]model.AnnotationSet, error) {
	var annotations []model.AnnotationSet
	if err := repository.db.Preload("Annotator").
		Where("dataset_id = ? AND schema_id = ? AND annotator_id = ? AND item_key = ?",
			annotation.DatasetID, annotation.SchemaID, annotation.AnnotatorID, annotation.ItemKey).
		Order("id ASC").Find(&annotations).Error; err != nil {
		return nil, fmt.Errorf("list annotation version chain: %w", err)
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

// Supersede moves a locked or compared result out of the current revision in a
// conditional update. Any concurrent lock transition makes this fail.
func (repository *AnnotationSetRepository) Supersede(id uint) error {
	result := repository.db.Model(&model.AnnotationSet{}).
		Where("id = ? AND annotation_state IN ?", id, []string{"locked", "compared"}).
		Update("annotation_state", "superseded")
	if result.Error != nil {
		return fmt.Errorf("supersede annotation set: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}
