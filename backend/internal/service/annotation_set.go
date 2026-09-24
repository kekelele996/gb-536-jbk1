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

type AnnotationSetService struct {
	db            *gorm.DB
	repository    *repository.AnnotationSetRepository
	datasets      *repository.CorpusDatasetRepository
	schemas       *repository.AnnotationSchemaRepository
	adjudications *repository.AdjudicationCaseRepository
	system        *SystemService
}

func NewAnnotationSetService(db *gorm.DB, repository *repository.AnnotationSetRepository, datasets *repository.CorpusDatasetRepository, schemas *repository.AnnotationSchemaRepository, adjudications *repository.AdjudicationCaseRepository, system *SystemService) *AnnotationSetService {
	return &AnnotationSetService{db: db, repository: repository, datasets: datasets, schemas: schemas, adjudications: adjudications, system: system}
}

func (service *AnnotationSetService) Create(request dto.CreateAnnotationSetRequest, actor dto.Actor, requestID string) (dto.AnnotationSetResponse, error) {
	dataset, err := service.datasets.Get(request.DatasetID)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("corpus dataset", err)
	}
	if dataset.DatasetState != constants.DatasetFrozen {
		return dto.AnnotationSetResponse{}, Conflict("dataset_not_frozen", "annotations require a frozen dataset version", repository.ErrStateConflict)
	}
	schema, err := service.schemas.Get(request.SchemaID)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation schema", err)
	}
	if schema.DatasetID != dataset.ID || schema.SchemaState != constants.SchemaPublished {
		return dto.AnnotationSetResponse{}, Unprocessable("incompatible_schema", "schema must be published for the selected dataset", nil)
	}
	labels, err := normalizeAndValidateLabels(request.Labels, schema)
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	labelsJSON, _ := json.Marshal(labels)
	annotation := model.AnnotationSet{
		DatasetID: dataset.ID, SchemaID: schema.ID, AnnotatorID: actor.ID,
		ItemKey: strings.TrimSpace(request.ItemKey), LabelsJSON: string(labelsJSON),
		SourceChecksum: strings.ToLower(request.SourceChecksum), AnnotationState: constants.AnnotationDraft,
		QualityNote: strings.TrimSpace(request.QualityNote),
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		annotations := service.repository.WithDB(tx)
		if createErr := annotations.Create(&annotation); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_annotation_revision", "the same annotation payload already exists for this annotator", createErr)
			}
			return Internal("could not create annotation set", createErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.created", "annotation_set", auditID(annotation.ID),
			map[string]any{"dataset_id": annotation.DatasetID, "schema_id": annotation.SchemaID},
			nil, annotationSummary(annotation, len(labels)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(annotation.ID)
}

// Replace starts a trackable replacement for a locked or compared result: the
// old version immediately leaves current comparisons and adjudications, while
// a new draft inherits the same annotator, dataset, schema and item. Repeating
// the request returns the same replacement draft.
func (service *AnnotationSetService) Replace(id uint, request dto.ReplaceAnnotationSetRequest, actor dto.Actor, requestID string) (dto.AnnotationSetResponse, error) {
	reason := strings.TrimSpace(request.Reason)
	if len(reason) < 8 {
		return dto.AnnotationSetResponse{}, BadRequest("invalid_replacement_reason", "a replacement reason of at least 8 characters is required")
	}
	previous, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	if previous.AnnotatorID != actor.ID && actor.Role != constants.RoleAdmin {
		return dto.AnnotationSetResponse{}, Forbidden("only the owning annotator or an admin can start a replacement")
	}
	// Idempotency: if the result already belongs to a replacement chain, a
	// repeated request for the same old result yields the same draft.
	if chain, chainErr := service.repository.VersionChain(previous); chainErr == nil {
		if successor := findSuccessor(chain, previous.ID); successor != nil {
			response, responseErr := service.responseWithChain(*successor)
			if responseErr != nil {
				return dto.AnnotationSetResponse{}, responseErr
			}
			response.Reused = true
			return response, nil
		}
	} else {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation version chain", chainErr)
	}
	if !constants.CanReplaceAnnotation(previous.AnnotationState) {
		return dto.AnnotationSetResponse{}, Conflict("invalid_annotation_replacement", "only locked or compared results can be replaced", repository.ErrStateConflict)
	}
	dataset, err := service.datasets.Get(previous.DatasetID)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("corpus dataset", err)
	}
	if dataset.DatasetState != constants.DatasetFrozen {
		return dto.AnnotationSetResponse{}, Conflict("dataset_not_frozen", "replacements require a frozen dataset version", repository.ErrStateConflict)
	}
	schema, err := service.schemas.Get(previous.SchemaID)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation schema", err)
	}
	labels := []dto.AnnotationLabel{}
	if err := json.Unmarshal([]byte(previous.LabelsJSON), &labels); err != nil {
		return dto.AnnotationSetResponse{}, Internal("previous annotation labels could not be decoded", err)
	}
	if _, err := normalizeAndValidateLabels(labels, schema); err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	labelsJSON, _ := json.Marshal(labels)
	replacement := model.AnnotationSet{
		DatasetID: previous.DatasetID, SchemaID: previous.SchemaID, AnnotatorID: previous.AnnotatorID,
		ItemKey: previous.ItemKey, LabelsJSON: string(labelsJSON),
		SourceChecksum: previous.SourceChecksum, AnnotationState: constants.AnnotationDraft,
		SupersedesID: &previous.ID, ReplaceReason: reason, QualityNote: previous.QualityNote,
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		annotations := service.repository.WithDB(tx)
		cases := service.adjudications.WithDB(tx)
		// Move the old version out first so the partial current-revision index
		// permits the new draft to share its item and source checksum.
		if transitionErr := annotations.Supersede(previous.ID); transitionErr != nil {
			return Conflict("state_conflict", "annotation changed concurrently while starting replacement", transitionErr)
		}
		if createErr := annotations.Create(&replacement); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_annotation_revision", "a replacement draft already exists for this result", createErr)
			}
			return Internal("could not create replacement draft", createErr)
		}
		if caseErr := service.retireReferencedCases(tx, cases, previous, reason, actor, requestID); caseErr != nil {
			return caseErr
		}
		after := previous
		after.AnnotationState = constants.AnnotationSuperseded
		if auditErr := service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.replacement_started", "annotation_set", auditID(previous.ID),
			map[string]any{"replacement_id": replacement.ID, "reason_present": true, "reason_length": len(reason)},
			annotationSummary(previous, labelCount(previous.LabelsJSON)), annotationSummary(after, labelCount(previous.LabelsJSON))); auditErr != nil {
			return auditErr
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.created", "annotation_set", auditID(replacement.ID),
			map[string]any{"dataset_id": replacement.DatasetID, "schema_id": replacement.SchemaID, "supersedes_id": previous.ID},
			nil, annotationSummary(replacement, len(labels)))
	})
	if err != nil {
		if repository.IsUniqueViolation(err) {
			// Concurrent replacement lost the race: resolve the winning draft so
			// a repeated call still returns the same replacement.
			if existing, findErr := service.repository.FindReplacement(previous.ID); findErr == nil {
				response, responseErr := service.responseWithChain(existing)
				if responseErr == nil {
					response.Reused = true
					return response, nil
				}
			}
		}
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(replacement.ID)
}

func (service *AnnotationSetService) Get(id uint) (dto.AnnotationSetResponse, error) {
	annotation, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	return service.responseWithChain(annotation)
}

func (service *AnnotationSetService) responseWithChain(annotation model.AnnotationSet) (dto.AnnotationSetResponse, error) {
	response := annotationResponse(annotation)
	chain, err := service.repository.VersionChain(annotation)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation version chain", err)
	}
	response.VersionChain = make([]dto.AnnotationVersionChainItem, 0, len(chain))
	for _, revision := range chain {
		response.VersionChain = append(response.VersionChain, dto.AnnotationVersionChainItem{
			ID: revision.ID, AnnotationState: revision.AnnotationState, SupersedesID: revision.SupersedesID,
			ReplaceReason: revision.ReplaceReason, QualityNote: revision.QualityNote,
			SubmittedAt: revision.SubmittedAt, CreatedAt: revision.CreatedAt, UpdatedAt: revision.UpdatedAt,
		})
	}
	return response, nil
}

func (service *AnnotationSetService) List(page, pageSize int, datasetID uint, itemKey, state string) ([]dto.AnnotationSetResponse, dto.PageMeta, error) {
	annotations, total, err := service.repository.List(page, pageSize, datasetID, itemKey, state)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list annotation sets", err)
	}
	responses := make([]dto.AnnotationSetResponse, 0, len(annotations))
	for _, annotation := range annotations {
		responses = append(responses, annotationResponse(annotation))
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *AnnotationSetService) Update(id uint, request dto.UpdateAnnotationSetRequest, actor dto.Actor, requestID string) (dto.AnnotationSetResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	if before.AnnotatorID != actor.ID {
		return dto.AnnotationSetResponse{}, Forbidden("annotators can edit only their own draft")
	}
	labels, err := normalizeAndValidateLabels(request.Labels, before.Schema)
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	labelsJSON, _ := json.Marshal(labels)
	var after model.AnnotationSet
	err = service.db.Transaction(func(tx *gorm.DB) error {
		annotations := service.repository.WithDB(tx)
		if updateErr := annotations.UpdateDraft(id, actor.ID, before.UpdatedAt, string(labelsJSON), strings.TrimSpace(request.QualityNote)); updateErr != nil {
			if errors.Is(updateErr, repository.ErrStateConflict) {
				return Conflict("state_conflict", "only an unchanged owned draft can be edited", updateErr)
			}
			return Internal("could not update annotation draft", updateErr)
		}
		var reloadErr error
		after, reloadErr = annotations.Get(id)
		if reloadErr != nil {
			return Internal("could not reload annotation set", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.updated", "annotation_set", auditID(id),
			map[string]any{"label_count": len(labels)}, annotationSummary(before, labelCount(before.LabelsJSON)), annotationSummary(after, len(labels)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(id)
}

func (service *AnnotationSetService) Transition(id uint, request dto.AnnotationTransitionRequest, actor dto.Actor, requestID string) (dto.AnnotationSetResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	if !constants.CanTransitionAnnotation(before.AnnotationState, request.TargetState) {
		return dto.AnnotationSetResponse{}, Conflict("invalid_annotation_transition",
			"annotation transition is not allowed from "+before.AnnotationState+" to "+request.TargetState, repository.ErrStateConflict)
	}
	if err := authorizeAnnotationTransition(before, request.TargetState, actor); err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		if transitionErr := service.repository.WithDB(tx).Transition(id, before.AnnotationState, request.TargetState); transitionErr != nil {
			return Conflict("state_conflict", "annotation state changed concurrently", transitionErr)
		}
		after := before
		after.AnnotationState = request.TargetState
		auditParameters := map[string]any{"reason_present": strings.TrimSpace(request.Reason) != ""}
		if before.SupersedesID != nil && request.TargetState == constants.AnnotationLocked {
			auditParameters["supersedes_id"] = *before.SupersedesID
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set."+request.TargetState, "annotation_set", auditID(id),
			auditParameters,
			annotationSummary(before, labelCount(before.LabelsJSON)), annotationSummary(after, labelCount(before.LabelsJSON)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(id)
}

// retireReferencedCases rewrites the adjudication history that froze the
// replaced annotation set: unfinished cases become pending_recompute, while
// already accepted decisions stay accepted but are tagged as superseded
// evidence and remain view-only.
func (service *AnnotationSetService) retireReferencedCases(tx *gorm.DB, cases *repository.AdjudicationCaseRepository, previous model.AnnotationSet, reason string, actor dto.Actor, requestID string) error {
	referenced, err := cases.CasesForItem(previous.DatasetID, previous.ItemKey)
	if err != nil {
		return Internal("could not load adjudication cases referencing the annotation", err)
	}
	for _, adjudication := range referenced {
		ids := []uint{}
		if decodeErr := json.Unmarshal([]byte(adjudication.AnnotationSetIDsJSON), &ids); decodeErr != nil {
			return Internal("adjudication annotation snapshot could not be decoded", decodeErr)
		}
		if !containsUint(ids, previous.ID) {
			continue
		}
		staleIDs := []uint{}
		if adjudication.SupersededSetIDsJSON != "" {
			_ = json.Unmarshal([]byte(adjudication.SupersededSetIDsJSON), &staleIDs)
		}
		if !containsUint(staleIDs, previous.ID) {
			staleIDs = append(staleIDs, previous.ID)
		}
		staleJSON, _ := json.Marshal(staleIDs)
		summary := caseReferenceSummary(adjudication, previous.ID)
		if constants.CaseIsActive(adjudication.CaseState) {
			if markErr := cases.MarkPendingRecompute(adjudication.ID, string(staleJSON)); markErr != nil {
				return Conflict("state_conflict", "adjudication case changed concurrently while retiring evidence", markErr)
			}
			after := adjudication
			after.CaseState = constants.CasePendingRecompute
			if auditErr := service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case.pending_recompute", "adjudication_case", auditID(adjudication.ID),
				map[string]any{"superseded_annotation_set_id": previous.ID, "replacement_reason_present": strings.TrimSpace(reason) != ""},
				caseSummary(adjudication), caseSummary(after)); auditErr != nil {
				return auditErr
			}
		} else if adjudication.CaseState == constants.CaseAccepted {
			if recordErr := cases.RecordStaleAcceptedHistory(adjudication.ID, string(staleJSON)); recordErr != nil {
				return Internal("could not tag accepted adjudication history", recordErr)
			}
			if auditErr := service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case.evidence_superseded", "adjudication_case", auditID(adjudication.ID),
				map[string]any{"superseded_annotation_set_id": previous.ID, "replacement_reason_present": strings.TrimSpace(reason) != ""},
				summary, summary); auditErr != nil {
				return auditErr
			}
		}
		// Already pending_recompute: the stale id is recorded when the case is
		// first marked, so no second write is needed.
	}
	return nil
}

func findSuccessor(chain []model.AnnotationSet, previousID uint) *model.AnnotationSet {
	for index := range chain {
		if chain[index].SupersedesID != nil && *chain[index].SupersedesID == previousID {
			return &chain[index]
		}
	}
	return nil
}

func containsUint(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func caseReferenceSummary(adjudication model.AdjudicationCase, annotationID uint) map[string]any {
	summary := caseSummary(adjudication)
	summary["superseded_annotation_set_id"] = annotationID
	return summary
}

func authorizeAnnotationTransition(annotation model.AnnotationSet, target string, actor dto.Actor) error {
	ownerAction := target == constants.AnnotationSubmitted || target == constants.AnnotationDraft
	if ownerAction && annotation.AnnotatorID != actor.ID {
		return Forbidden("annotators can submit or resume only their own annotation")
	}
	if target == constants.AnnotationLocked || target == constants.AnnotationReturned || target == constants.AnnotationCompared {
		if actor.Role != constants.RoleDataManager && actor.Role != constants.RoleAdjudicator && actor.Role != constants.RoleAdmin {
			return Forbidden("this annotation transition requires data manager or adjudicator authority")
		}
	}
	return nil
}

func normalizeAndValidateLabels(labels []dto.AnnotationLabel, schema model.AnnotationSchema) ([]dto.AnnotationLabel, error) {
	definitions := []dto.LabelDefinition{}
	if err := json.Unmarshal([]byte(schema.LabelDefinitionsJSON), &definitions); err != nil {
		return nil, Internal("schema label definitions could not be decoded", err)
	}
	allowed := map[string]string{}
	for _, definition := range definitions {
		allowed[strings.ToUpper(definition.Code)] = definition.TaskType
	}
	normalized := make([]dto.AnnotationLabel, 0, len(labels))
	seenClassification := map[string]bool{}
	for _, label := range labels {
		label.UnitKey = strings.TrimSpace(label.UnitKey)
		label.Label = strings.ToUpper(strings.TrimSpace(label.Label))
		taskType, exists := allowed[label.Label]
		if !exists {
			return nil, Unprocessable("unknown_label", "annotation uses a label not defined by the schema", nil)
		}
		if taskType == "span" && label.End <= label.Start {
			return nil, Unprocessable("invalid_span", "span labels require end greater than start", nil)
		}
		if taskType == "classification" {
			if label.Start != 0 || label.End != 0 {
				return nil, Unprocessable("invalid_classification", "classification labels cannot contain character offsets", nil)
			}
			if seenClassification[label.UnitKey] {
				return nil, Unprocessable("duplicate_unit_rating", "classification units may be rated once per annotation set", nil)
			}
			seenClassification[label.UnitKey] = true
		}
		normalized = append(normalized, label)
	}
	return normalized, nil
}

func annotationResponse(annotation model.AnnotationSet) dto.AnnotationSetResponse {
	labels := []dto.AnnotationLabel{}
	_ = json.Unmarshal([]byte(annotation.LabelsJSON), &labels)
	return dto.AnnotationSetResponse{
		ID: annotation.ID, DatasetID: annotation.DatasetID, DatasetCode: annotation.Dataset.DatasetCode,
		SchemaID: annotation.SchemaID, SchemaCode: annotation.Schema.SchemaCode, SchemaVersion: annotation.Schema.Version,
		AnnotatorID: annotation.AnnotatorID, Annotator: annotation.Annotator.Username, ItemKey: annotation.ItemKey,
		Labels: labels, SourceChecksum: annotation.SourceChecksum, AnnotationState: annotation.AnnotationState,
		SubmittedAt: annotation.SubmittedAt, SupersedesID: annotation.SupersedesID, ReplaceReason: annotation.ReplaceReason,
		QualityNote: annotation.QualityNote,
		CreatedAt:   annotation.CreatedAt, UpdatedAt: annotation.UpdatedAt,
	}
}

func labelCount(encoded string) int {
	labels := []dto.AnnotationLabel{}
	_ = json.Unmarshal([]byte(encoded), &labels)
	return len(labels)
}

func annotationSummary(annotation model.AnnotationSet, labels int) map[string]any {
	summary := map[string]any{
		"dataset_id": annotation.DatasetID, "schema_id": annotation.SchemaID, "annotator_id": annotation.AnnotatorID,
		"item_key": annotation.ItemKey, "annotation_state": annotation.AnnotationState,
		"label_count": labels, "source_checksum": annotation.SourceChecksum,
	}
	if annotation.SupersedesID != nil {
		summary["supersedes_id"] = *annotation.SupersedesID
	}
	return summary
}
