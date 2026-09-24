package dto

import "time"

type CreateCorpusDatasetRequest struct {
	DatasetCode       string `json:"dataset_code" validate:"required,min=3,max=64"`
	Name              string `json:"name" validate:"required,min=3,max=160"`
	Language          string `json:"language" validate:"required,min=2,max=24"`
	Domain            string `json:"domain" validate:"required,min=2,max=80"`
	DocumentCount     int    `json:"document_count" validate:"gte=0,lte=100000000"`
	ContentMaskPolicy string `json:"content_mask_policy" validate:"required,min=8,max=300"`
	OwnerTeam         string `json:"owner_team" validate:"required,min=2,max=120"`
}

type UpdateCorpusDatasetRequest struct {
	Name              string `json:"name" validate:"required,min=3,max=160"`
	Language          string `json:"language" validate:"required,min=2,max=24"`
	Domain            string `json:"domain" validate:"required,min=2,max=80"`
	DocumentCount     int    `json:"document_count" validate:"gte=0,lte=100000000"`
	ContentMaskPolicy string `json:"content_mask_policy" validate:"required,min=8,max=300"`
	OwnerTeam         string `json:"owner_team" validate:"required,min=2,max=120"`
	Version           int    `json:"version" validate:"required,gte=1"`
}

type CorpusDatasetResponse struct {
	ID                uint      `json:"id"`
	DatasetCode       string    `json:"dataset_code"`
	Name              string    `json:"name"`
	Language          string    `json:"language"`
	Domain            string    `json:"domain"`
	DocumentCount     int       `json:"document_count"`
	ContentMaskPolicy string    `json:"content_mask_policy"`
	DatasetState      string    `json:"dataset_state"`
	Version           int       `json:"version"`
	OwnerTeam         string    `json:"owner_team"`
	SchemaCount       int64     `json:"schema_count"`
	AnnotationCount   int64     `json:"annotation_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
