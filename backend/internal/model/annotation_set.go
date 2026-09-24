package model

import "time"

type AnnotationSet struct {
	ID                uint             `gorm:"primaryKey"`
	DatasetID         uint             `gorm:"index:idx_annotation_identity,priority:1;index;not null"`
	SchemaID          uint             `gorm:"index:idx_annotation_identity,priority:2;not null"`
	AnnotatorID       uint             `gorm:"index:idx_annotation_identity,priority:3;index;not null"`
	ItemKey           string           `gorm:"size:120;index:idx_annotation_identity,priority:4;index;not null"`
	LabelsJSON        string           `gorm:"type:text;not null"`
	SourceChecksum    string           `gorm:"size:64;not null"`
	AnnotationState   string           `gorm:"size:24;index;not null"`
	SubmittedAt       *time.Time       `gorm:"index"`
	SupersedesID      *uint            `gorm:"uniqueIndex:idx_annotation_open_revision;index"`
	ReplacementReason string           `gorm:"size:600;not null;default:''"`
	QualityNote       string           `gorm:"size:600;not null"`
	CreatedAt         time.Time        `gorm:"not null"`
	UpdatedAt         time.Time        `gorm:"not null"`
	Dataset           CorpusDataset    `gorm:"foreignKey:DatasetID"`
	Schema            AnnotationSchema `gorm:"foreignKey:SchemaID"`
	Annotator         User             `gorm:"foreignKey:AnnotatorID"`
}
