package dto

import "time"

type AnnotationLabel struct {
	UnitKey string `json:"unit_key" validate:"required,min=1,max=120"`
	Label   string `json:"label" validate:"required,min=1,max=40"`
	Start   int    `json:"start,omitempty" validate:"gte=0"`
	End     int    `json:"end,omitempty" validate:"gte=0"`
}

func (label AnnotationLabel) IsSpan() bool { return label.End > label.Start }

type CreateAnnotationSetRequest struct {
	DatasetID      uint              `json:"dataset_id" validate:"required"`
	SchemaID       uint              `json:"schema_id" validate:"required"`
	ItemKey        string            `json:"item_key" validate:"required,min=2,max=120"`
	Labels         []AnnotationLabel `json:"labels" validate:"required,min=1,dive"`
	SourceChecksum string            `json:"source_checksum" validate:"required,len=64,hexadecimal"`
	QualityNote    string            `json:"quality_note" validate:"max=600"`
	SupersedesID   *uint             `json:"supersedes_id"`
}

type UpdateAnnotationSetRequest struct {
	Labels      []AnnotationLabel `json:"labels" validate:"required,min=1,dive"`
	QualityNote string            `json:"quality_note" validate:"max=600"`
}

type AnnotationTransitionRequest struct {
	TargetState string `json:"target_state" validate:"required"`
	Reason      string `json:"reason" validate:"max=300"`
}

type AnnotationSetResponse struct {
	ID              uint              `json:"id"`
	DatasetID       uint              `json:"dataset_id"`
	DatasetCode     string            `json:"dataset_code"`
	SchemaID        uint              `json:"schema_id"`
	SchemaCode      string            `json:"schema_code"`
	SchemaVersion   int               `json:"schema_version"`
	AnnotatorID     uint              `json:"annotator_id"`
	Annotator       string            `json:"annotator"`
	ItemKey         string            `json:"item_key"`
	Labels          []AnnotationLabel `json:"labels"`
	SourceChecksum  string            `json:"source_checksum"`
	AnnotationState string            `json:"annotation_state"`
	SubmittedAt     *time.Time        `json:"submitted_at"`
	SupersedesID    *uint             `json:"supersedes_id"`
	QualityNote     string            `json:"quality_note"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}
