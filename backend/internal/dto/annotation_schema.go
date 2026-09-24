package dto

import (
	"encoding/json"
	"time"
)

type LabelDefinition struct {
	Code        string `json:"code" validate:"required,min=1,max=40"`
	DisplayName string `json:"display_name" validate:"required,min=1,max=80"`
	TaskType    string `json:"task_type" validate:"required,oneof=classification span"`
	Description string `json:"description" validate:"required,min=4,max=240"`
}

type MaskedExample struct {
	ItemKey    string          `json:"item_key" validate:"required,max=120"`
	MaskedText string          `json:"masked_text" validate:"required,max=500"`
	Labels     json.RawMessage `json:"labels" validate:"required"`
}

type CreateAnnotationSchemaRequest struct {
	DatasetID        uint              `json:"dataset_id" validate:"required"`
	SchemaCode       string            `json:"schema_code" validate:"required,min=3,max=64"`
	Version          int               `json:"version" validate:"required,gte=1,lte=9999"`
	LabelDefinitions []LabelDefinition `json:"label_definitions" validate:"required,min=2,dive"`
	SpanPolicy       string            `json:"span_policy" validate:"required,min=8,max=300"`
	OverlapPolicy    string            `json:"overlap_policy" validate:"required,min=8,max=300"`
	Examples         []MaskedExample   `json:"examples" validate:"max=20,dive"`
}

type UpdateAnnotationSchemaRequest struct {
	LabelDefinitions []LabelDefinition `json:"label_definitions" validate:"required,min=2,dive"`
	SpanPolicy       string            `json:"span_policy" validate:"required,min=8,max=300"`
	OverlapPolicy    string            `json:"overlap_policy" validate:"required,min=8,max=300"`
	Examples         []MaskedExample   `json:"examples" validate:"max=20,dive"`
	Version          int               `json:"version" validate:"required,gte=1"`
}

type CopyAnnotationSchemaRequest struct {
	Version int `json:"version" validate:"required,gte=1,lte=9999"`
}

type SchemaTransitionRequest struct {
	TargetState string `json:"target_state" validate:"required"`
}

type AnnotationSchemaResponse struct {
	ID               uint              `json:"id"`
	DatasetID        uint              `json:"dataset_id"`
	DatasetCode      string            `json:"dataset_code"`
	SchemaCode       string            `json:"schema_code"`
	Version          int               `json:"version"`
	LabelDefinitions []LabelDefinition `json:"label_definitions"`
	SpanPolicy       string            `json:"span_policy"`
	OverlapPolicy    string            `json:"overlap_policy"`
	Examples         []MaskedExample   `json:"examples"`
	SchemaState      string            `json:"schema_state"`
	CreatedBy        uint              `json:"created_by"`
	PublishedAt      *time.Time        `json:"published_at"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
