package model

import "time"

type AnnotationSet struct {
	ID              uint             `gorm:"primaryKey"`
	DatasetID       uint             `gorm:"index:idx_annotation_item;not null"`
	SchemaID        uint             `gorm:"index:idx_annotation_item;not null"`
	AnnotatorID     uint             `gorm:"index:idx_annotation_item;not null"`
	ItemKey         string           `gorm:"size:120;index:idx_annotation_item;index;not null"`
	LabelsJSON      string           `gorm:"type:text;not null"`
	SourceChecksum  string           `gorm:"size:64;index;not null"`
	AnnotationState string           `gorm:"size:24;index;not null"`
	SubmittedAt     *time.Time       `gorm:"index"`
	SupersedesID    *uint            `gorm:"uniqueIndex:idx_annotation_supersedes;index"`
	ReplaceReason   string           `gorm:"size:300;not null;default:''"`
	QualityNote     string           `gorm:"size:600;not null"`
	CreatedAt       time.Time        `gorm:"not null"`
	UpdatedAt       time.Time        `gorm:"not null"`
	Dataset         CorpusDataset    `gorm:"foreignKey:DatasetID"`
	Schema          AnnotationSchema `gorm:"foreignKey:SchemaID"`
	Annotator       User             `gorm:"foreignKey:AnnotatorID"`
}
