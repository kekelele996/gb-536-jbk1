package service

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/model"
	"corpus-annotation-agreement-control/backend/internal/repository"
)

type AnnotationSchemaService struct {
	db         *gorm.DB
	repository *repository.AnnotationSchemaRepository
	datasets   *repository.CorpusDatasetRepository
	system     *SystemService
}

func NewAnnotationSchemaService(db *gorm.DB, repository *repository.AnnotationSchemaRepository, datasets *repository.CorpusDatasetRepository, system *SystemService) *AnnotationSchemaService {
	return &AnnotationSchemaService{db: db, repository: repository, datasets: datasets, system: system}
}

func (service *AnnotationSchemaService) Create(request dto.CreateAnnotationSchemaRequest, actor dto.Actor, requestID string) (dto.AnnotationSchemaResponse, error) {
	if err := validateSchemaDefinitions(request.LabelDefinitions, request.Examples); err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	dataset, err := service.datasets.Get(request.DatasetID)
	if err != nil {
		return dto.AnnotationSchemaResponse{}, MapRepositoryError("corpus dataset", err)
	}
	if dataset.DatasetState == constants.DatasetArchived {
		return dto.AnnotationSchemaResponse{}, Conflict("dataset_archived", "cannot add a schema to an archived dataset", repository.ErrStateConflict)
	}
	definitionsJSON, _ := json.Marshal(request.LabelDefinitions)
	examplesJSON, _ := json.Marshal(request.Examples)
	schema := model.AnnotationSchema{
		DatasetID: request.DatasetID, SchemaCode: strings.ToUpper(strings.TrimSpace(request.SchemaCode)),
		Version: request.Version, LabelDefinitionsJSON: string(definitionsJSON),
		SpanPolicy: strings.TrimSpace(request.SpanPolicy), OverlapPolicy: strings.TrimSpace(request.OverlapPolicy),
		ExamplesJSON: string(examplesJSON), SchemaState: constants.SchemaDraft, CreatedBy: actor.ID,
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		if createErr := service.repository.WithDB(tx).Create(&schema); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_schema_version", "schema_code and version already exist for this dataset", createErr)
			}
			return Internal("could not create annotation schema", createErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_schema.created", "annotation_schema", auditID(schema.ID),
			map[string]any{"dataset_id": schema.DatasetID}, nil, schemaSummary(schema))
	})
	if err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	return service.Get(schema.ID)
}

func (service *AnnotationSchemaService) Get(id uint) (dto.AnnotationSchemaResponse, error) {
	schema, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSchemaResponse{}, MapRepositoryError("annotation schema", err)
	}
	return schemaResponse(schema), nil
}

func (service *AnnotationSchemaService) List(page, pageSize int, datasetID uint, state, schemaCode string) ([]dto.AnnotationSchemaResponse, dto.PageMeta, error) {
	schemas, total, err := service.repository.List(page, pageSize, datasetID, state, schemaCode)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list annotation schemas", err)
	}
	responses := make([]dto.AnnotationSchemaResponse, 0, len(schemas))
	for _, schema := range schemas {
		responses = append(responses, schemaResponse(schema))
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *AnnotationSchemaService) Update(id uint, request dto.UpdateAnnotationSchemaRequest, actor dto.Actor, requestID string) (dto.AnnotationSchemaResponse, error) {
	if err := validateSchemaDefinitions(request.LabelDefinitions, request.Examples); err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSchemaResponse{}, MapRepositoryError("annotation schema", err)
	}
	if before.Version != request.Version {
		return dto.AnnotationSchemaResponse{}, Conflict("version_conflict", "schema version does not match", repository.ErrVersionConflict)
	}
	definitionsJSON, _ := json.Marshal(request.LabelDefinitions)
	examplesJSON, _ := json.Marshal(request.Examples)
	updated := before
	updated.LabelDefinitionsJSON, updated.ExamplesJSON = string(definitionsJSON), string(examplesJSON)
	updated.SpanPolicy, updated.OverlapPolicy = strings.TrimSpace(request.SpanPolicy), strings.TrimSpace(request.OverlapPolicy)
	var after model.AnnotationSchema
	err = service.db.Transaction(func(tx *gorm.DB) error {
		schemas := service.repository.WithDB(tx)
		if updateErr := schemas.UpdateDraft(&updated, before.UpdatedAt); updateErr != nil {
			if errors.Is(updateErr, repository.ErrVersionConflict) {
				return Conflict("version_conflict", "only the current draft schema may be edited", updateErr)
			}
			return Internal("could not update annotation schema", updateErr)
		}
		var reloadErr error
		after, reloadErr = schemas.Get(id)
		if reloadErr != nil {
			return Internal("could not reload annotation schema", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_schema.updated", "annotation_schema", auditID(id),
			map[string]any{"version": request.Version}, schemaSummary(before), schemaSummary(after))
	})
	if err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	return schemaResponse(after), nil
}

func (service *AnnotationSchemaService) Copy(id uint, request dto.CopyAnnotationSchemaRequest, actor dto.Actor, requestID string) (dto.AnnotationSchemaResponse, error) {
	source, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSchemaResponse{}, MapRepositoryError("annotation schema", err)
	}
	copy := model.AnnotationSchema{
		DatasetID: source.DatasetID, SchemaCode: source.SchemaCode, Version: request.Version,
		LabelDefinitionsJSON: source.LabelDefinitionsJSON, SpanPolicy: source.SpanPolicy,
		OverlapPolicy: source.OverlapPolicy, ExamplesJSON: source.ExamplesJSON,
		SchemaState: constants.SchemaDraft, CreatedBy: actor.ID,
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		if createErr := service.repository.WithDB(tx).Create(&copy); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_schema_version", "requested schema version already exists", createErr)
			}
			return Internal("could not copy annotation schema", createErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_schema.copied", "annotation_schema", auditID(copy.ID),
			map[string]any{"source_schema_id": source.ID, "source_version": source.Version}, nil, schemaSummary(copy))
	})
	if err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	return service.Get(copy.ID)
}

func (service *AnnotationSchemaService) Transition(id uint, target string, actor dto.Actor, requestID string) (dto.AnnotationSchemaResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSchemaResponse{}, MapRepositoryError("annotation schema", err)
	}
	if !constants.CanTransitionSchema(before.SchemaState, target) {
		return dto.AnnotationSchemaResponse{}, Conflict("invalid_schema_transition",
			"schema transition is not allowed from "+before.SchemaState+" to "+target, repository.ErrStateConflict)
	}
	var after model.AnnotationSchema
	err = service.db.Transaction(func(tx *gorm.DB) error {
		schemas := service.repository.WithDB(tx)
		if transitionErr := schemas.Transition(id, before.SchemaState, target); transitionErr != nil {
			return Conflict("state_conflict", "schema state changed concurrently", transitionErr)
		}
		var reloadErr error
		after, reloadErr = schemas.Get(id)
		if reloadErr != nil {
			return Internal("could not reload annotation schema", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_schema."+target, "annotation_schema", auditID(id),
			nil, schemaSummary(before), schemaSummary(after))
	})
	if err != nil {
		return dto.AnnotationSchemaResponse{}, err
	}
	return schemaResponse(after), nil
}

func validateSchemaDefinitions(definitions []dto.LabelDefinition, examples []dto.MaskedExample) error {
	seen := map[string]bool{}
	for _, definition := range definitions {
		code := strings.ToUpper(strings.TrimSpace(definition.Code))
		if seen[code] {
			return Unprocessable("duplicate_label_definition", "label definition codes must be unique", nil)
		}
		seen[code] = true
		if definition.TaskType != "classification" && definition.TaskType != "span" {
			return Unprocessable("invalid_label_definition", "task_type must be classification or span", nil)
		}
	}
	for _, example := range examples {
		if !strings.Contains(example.MaskedText, "***") && !strings.Contains(example.MaskedText, "[MASK]") {
			return Unprocessable("unmasked_example", "schema examples must visibly mask source content", nil)
		}
	}
	return nil
}

func schemaResponse(schema model.AnnotationSchema) dto.AnnotationSchemaResponse {
	definitions := []dto.LabelDefinition{}
	examples := []dto.MaskedExample{}
	_ = json.Unmarshal([]byte(schema.LabelDefinitionsJSON), &definitions)
	_ = json.Unmarshal([]byte(schema.ExamplesJSON), &examples)
	return dto.AnnotationSchemaResponse{
		ID: schema.ID, DatasetID: schema.DatasetID, DatasetCode: schema.Dataset.DatasetCode,
		SchemaCode: schema.SchemaCode, Version: schema.Version, LabelDefinitions: definitions,
		SpanPolicy: schema.SpanPolicy, OverlapPolicy: schema.OverlapPolicy, Examples: examples,
		SchemaState: schema.SchemaState, CreatedBy: schema.CreatedBy, PublishedAt: schema.PublishedAt,
		CreatedAt: schema.CreatedAt, UpdatedAt: schema.UpdatedAt,
	}
}

func schemaSummary(schema model.AnnotationSchema) map[string]any {
	definitions := []dto.LabelDefinition{}
	_ = json.Unmarshal([]byte(schema.LabelDefinitionsJSON), &definitions)
	return map[string]any{
		"dataset_id": schema.DatasetID, "schema_code": schema.SchemaCode, "version": schema.Version,
		"schema_state": schema.SchemaState, "label_definition_count": len(definitions),
		"span_policy_configured": schema.SpanPolicy != "", "overlap_policy_configured": schema.OverlapPolicy != "",
	}
}
