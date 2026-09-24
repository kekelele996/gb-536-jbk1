package model

import "time"

type AdjudicationCase struct {
	ID                     uint          `gorm:"primaryKey"`
	DatasetID              uint          `gorm:"index;not null"`
	ItemKey                string        `gorm:"size:120;index;not null"`
	AnnotationSetIDsJSON   string        `gorm:"type:text;not null"`
	AgreementMetric        string        `gorm:"size:40;index;not null"`
	AgreementScore         float64       `gorm:"not null"`
	ObservedAgreement      float64       `gorm:"not null"`
	ChanceAgreement        float64       `gorm:"not null"`
	SampleSize             int           `gorm:"not null"`
	CoderCount             int           `gorm:"not null"`
	MissingValueCount      int           `gorm:"not null"`
	Applicability          string        `gorm:"size:500;not null"`
	DisagreementType       string        `gorm:"size:24;index;not null"`
	ConfusionSnapshotJSON  string        `gorm:"type:text;not null"`
	EvidenceSnapshotJSON   string        `gorm:"type:text;not null"`
	ClusterKey             string        `gorm:"size:180;index;not null"`
	CaseState              string        `gorm:"size:24;index;not null"`
	FinalLabelsJSON        string        `gorm:"type:text;not null"`
	Rationale              string        `gorm:"type:text;not null"`
	AdjudicatorID          *uint         `gorm:"index"`
	ReviewedBy             *uint         `gorm:"index"`
	DecidedAt              *time.Time    `gorm:"index"`
	InputHash              string        `gorm:"size:64;index;not null"`
	AlgorithmVersion       string        `gorm:"size:80;index;not null"`
	IdempotencyKey         string        `gorm:"size:120;uniqueIndex;not null"`
	DecisionIdempotencyKey *string       `gorm:"size:120;uniqueIndex"`
	ReopenCount            int           `gorm:"not null;default:0"`
	CreatedBy              uint          `gorm:"index;not null"`
	CreatedAt              time.Time     `gorm:"not null"`
	UpdatedAt              time.Time     `gorm:"not null"`
	Dataset                CorpusDataset `gorm:"foreignKey:DatasetID"`
}
