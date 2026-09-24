package model

import "time"

type AnnotationSchema struct {
	ID                   uint          `gorm:"primaryKey"`
	DatasetID            uint          `gorm:"index:idx_schema_version,unique;not null"`
	SchemaCode           string        `gorm:"size:64;index:idx_schema_version,unique;not null"`
	Version              int           `gorm:"index:idx_schema_version,unique;not null"`
	LabelDefinitionsJSON string        `gorm:"type:text;not null"`
	SpanPolicy           string        `gorm:"size:300;not null"`
	OverlapPolicy        string        `gorm:"size:300;not null"`
	ExamplesJSON         string        `gorm:"type:text;not null"`
	SchemaState          string        `gorm:"size:24;index;not null"`
	CreatedBy            uint          `gorm:"index;not null"`
	PublishedAt          *time.Time    `gorm:"index"`
	CreatedAt            time.Time     `gorm:"not null"`
	UpdatedAt            time.Time     `gorm:"not null"`
	Dataset              CorpusDataset `gorm:"foreignKey:DatasetID"`
}
