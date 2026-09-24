package model

import "time"

type AnnotationSet struct {
	ID              uint             `gorm:"primaryKey"`
	DatasetID       uint             `gorm:"uniqueIndex:idx_annotation_revision;not null"`
	SchemaID        uint             `gorm:"uniqueIndex:idx_annotation_revision;not null"`
	AnnotatorID     uint             `gorm:"uniqueIndex:idx_annotation_revision;not null"`
	ItemKey         string           `gorm:"size:120;uniqueIndex:idx_annotation_revision;index;not null"`
	LabelsJSON      string           `gorm:"type:text;not null"`
	SourceChecksum  string           `gorm:"size:64;uniqueIndex:idx_annotation_revision;not null"`
	AnnotationState string           `gorm:"size:24;index;not null"`
	SubmittedAt     *time.Time       `gorm:"index"`
	SupersedesID    *uint            `gorm:"index"`
	QualityNote     string           `gorm:"size:600;not null"`
	CreatedAt       time.Time        `gorm:"not null"`
	UpdatedAt       time.Time        `gorm:"not null"`
	Dataset         CorpusDataset    `gorm:"foreignKey:DatasetID"`
	Schema          AnnotationSchema `gorm:"foreignKey:SchemaID"`
	Annotator       User             `gorm:"foreignKey:AnnotatorID"`
}
