package model

import "time"

type CorpusDataset struct {
	ID                uint      `gorm:"primaryKey"`
	DatasetCode       string    `gorm:"size:64;uniqueIndex;not null"`
	Name              string    `gorm:"size:160;not null"`
	Language          string    `gorm:"size:24;index;not null"`
	Domain            string    `gorm:"size:80;index;not null"`
	DocumentCount     int       `gorm:"not null;default:0"`
	ContentMaskPolicy string    `gorm:"size:300;not null"`
	DatasetState      string    `gorm:"size:24;index;not null"`
	Version           int       `gorm:"not null;default:1"`
	OwnerTeam         string    `gorm:"size:120;index;not null"`
	CreatedBy         uint      `gorm:"index;not null"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}
