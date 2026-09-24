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
	db         *gorm.DB
	repository *repository.AnnotationSetRepository
	cases      *repository.AdjudicationCaseRepository
	datasets   *repository.CorpusDatasetRepository
	schemas    *repository.AnnotationSchemaRepository
	system     *SystemService
}

func NewAnnotationSetService(db *gorm.DB, repository *repository.AnnotationSetRepository, cases *repository.AdjudicationCaseRepository, datasets *repository.CorpusDatasetRepository, schemas *repository.AnnotationSchemaRepository, system *SystemService) *AnnotationSetService {
	return &AnnotationSetService{db: db, repository: repository, cases: cases, datasets: datasets, schemas: schemas, system: system}
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
		SupersedesID: request.SupersedesID, QualityNote: strings.TrimSpace(request.QualityNote),
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		annotations := service.repository.WithDB(tx)
		if request.SupersedesID != nil {
			previous, loadErr := annotations.Get(*request.SupersedesID)
			if loadErr != nil {
				return MapRepositoryError("superseded annotation set", loadErr)
			}
			if previous.AnnotatorID != actor.ID || previous.AnnotationState != constants.AnnotationCompared {
				return Conflict("invalid_supersedes", "only your compared annotation can be superseded", repository.ErrStateConflict)
			}
			if previous.DatasetID != dataset.ID || previous.ItemKey != annotation.ItemKey {
				return Unprocessable("invalid_supersedes", "superseded annotation must belong to the same dataset item", nil)
			}
			if _, successorErr := annotations.FindOpenReplacement(previous.ID); successorErr == nil {
				return Conflict("replacement_in_progress", "a replacement draft already exists for the selected annotation", repository.ErrStateConflict)
			} else if !errors.Is(successorErr, gorm.ErrRecordNotFound) {
				return Internal("could not verify replacement drafts", successorErr)
			}
			annotation.ReplacementReason = "superseded by direct revision"
		}
		if createErr := annotations.Create(&annotation); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_annotation_revision", "an open replacement draft already exists for this annotation", createErr)
			}
			return Internal("could not create annotation set", createErr)
		}
		if request.SupersedesID != nil {
			affected, markErr := service.cases.WithDB(tx).MarkPendingRecompute(*request.SupersedesID, constants.CaseStatesThatCanBeRecomputed())
			if markErr != nil {
				return Internal("could not retract adjudications referencing the replaced annotation", markErr)
			}
			_ = affected
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.created", "annotation_set", auditID(annotation.ID),
			map[string]any{"dataset_id": annotation.DatasetID, "schema_id": annotation.SchemaID, "supersedes_id": annotation.SupersedesID},
			nil, annotationSummary(annotation, len(labels)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(annotation.ID)
}

func (service *AnnotationSetService) Get(id uint) (dto.AnnotationSetResponse, error) {
	annotation, err := service.repository.Get(id)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	return service.responseWithSuccessor(annotation)
}

// Replace starts a tracked correction for a locked or compared result. It
// creates a fresh draft for the same annotator, dataset and item, seeded from
// the old result and pointing back to it. Repeating the request while a
// replacement draft is still open returns that same draft. Any not-yet-accepted
// adjudication referencing the old result is moved to pending_recompute;
// accepted adjudications remain read-only history.
func (service *AnnotationSetService) Replace(predecessorID uint, request dto.ReplaceAnnotationSetRequest, actor dto.Actor, requestID string) (dto.AnnotationSetResponse, error) {
	reason := strings.TrimSpace(request.Reason)
	previous, err := service.repository.Get(predecessorID)
	if err != nil {
		return dto.AnnotationSetResponse{}, MapRepositoryError("annotation set", err)
	}
	if previous.AnnotatorID != actor.ID && actor.Role != constants.RoleAdmin {
		return dto.AnnotationSetResponse{}, Forbidden("only the owning annotator or an admin can start a replacement")
	}
	if previous.AnnotationState != constants.AnnotationLocked && previous.AnnotationState != constants.AnnotationCompared {
		return dto.AnnotationSetResponse{}, Conflict("annotation_not_replaceable",
			"only locked or compared results can be replaced", repository.ErrStateConflict)
	}
	// Idempotency: a repeated replace returns the in-flight revision unchanged.
	if existing, findErr := service.repository.FindOpenReplacement(predecessorID); findErr == nil {
		return service.responseWithSuccessor(existing)
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return dto.AnnotationSetResponse{}, Internal("could not verify an existing replacement draft", findErr)
	}
	labels := []dto.AnnotationLabel{}
	if err := json.Unmarshal([]byte(previous.LabelsJSON), &labels); err != nil {
		return dto.AnnotationSetResponse{}, Internal("predecessor labels could not be decoded", err)
	}
	revision := model.AnnotationSet{
		DatasetID: previous.DatasetID, SchemaID: previous.SchemaID, AnnotatorID: previous.AnnotatorID,
		ItemKey: previous.ItemKey, LabelsJSON: previous.LabelsJSON, SourceChecksum: previous.SourceChecksum,
		AnnotationState: constants.AnnotationDraft, SupersedesID: &previous.ID,
		ReplacementReason: reason, QualityNote: strings.TrimSpace(previous.QualityNote),
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		annotations := service.repository.WithDB(tx)
		cases := service.cases.WithDB(tx)
		if createErr := annotations.Create(&revision); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("replacement_in_progress", "a replacement draft already exists for the selected annotation", createErr)
			}
			return Internal("could not create replacement draft", createErr)
		}
		if _, markErr := cases.MarkPendingRecompute(previous.ID, constants.CaseStatesThatCanBeRecomputed()); markErr != nil {
			return Internal("could not retract adjudications referencing the replaced annotation", markErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set.replacement_started", "annotation_set", auditID(revision.ID),
			map[string]any{"dataset_id": revision.DatasetID, "predecessor_id": previous.ID, "reason_present": reason != ""},
			annotationSummary(previous, len(labels)), annotationSummary(revision, len(labels)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(revision.ID)
}

// VersionChain returns every revision of one annotator's result for one item,
// oldest first, including superseded history and the recorded replacement
// reasons.
func (service *AnnotationSetService) VersionChain(datasetID uint, itemKey string, annotatorID uint) (dto.AnnotationVersionChainResponse, error) {
	itemKey = strings.TrimSpace(itemKey)
	if datasetID == 0 || itemKey == "" || annotatorID == 0 {
		return dto.AnnotationVersionChainResponse{}, BadRequest("invalid_chain_query", "dataset_id, item_key and annotator_id are required")
	}
	chain, err := service.repository.VersionChain(datasetID, itemKey, annotatorID)
	if err != nil {
		return dto.AnnotationVersionChainResponse{}, Internal("could not load annotation version chain", err)
	}
	links := make([]dto.AnnotationVersionLink, 0, len(chain))
	annotator := ""
	for _, annotation := range chain {
		annotator = annotation.Annotator.Username
		labels := []dto.AnnotationLabel{}
		_ = json.Unmarshal([]byte(annotation.LabelsJSON), &labels)
		links = append(links, dto.AnnotationVersionLink{
			ID: annotation.ID, AnnotationState: annotation.AnnotationState, Labels: labels,
			SourceChecksum: annotation.SourceChecksum, SupersedesID: annotation.SupersedesID,
			ReplacementReason: annotation.ReplacementReason, QualityNote: annotation.QualityNote,
			SubmittedAt: annotation.SubmittedAt, CreatedAt: annotation.CreatedAt, UpdatedAt: annotation.UpdatedAt,
		})
	}
	return dto.AnnotationVersionChainResponse{
		DatasetID: datasetID, ItemKey: itemKey, AnnotatorID: annotatorID, Annotator: annotator, Links: links,
	}, nil
}

func (service *AnnotationSetService) responseWithSuccessor(annotation model.AnnotationSet) (dto.AnnotationSetResponse, error) {
	response := annotationResponse(annotation)
	successorID, err := service.repository.SuccessorID(annotation.ID)
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	response.SupersededByID = successorID
	response.CurrentVersion = successorID == nil && annotation.AnnotationState != constants.AnnotationSuperseded
	return response, nil
}

func (service *AnnotationSetService) List(page, pageSize int, datasetID uint, itemKey, state string) ([]dto.AnnotationSetResponse, dto.PageMeta, error) {
	annotations, total, err := service.repository.List(page, pageSize, datasetID, itemKey, state)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list annotation sets", err)
	}
	responses := make([]dto.AnnotationSetResponse, 0, len(annotations))
	for _, annotation := range annotations {
		response := annotationResponse(annotation)
		successorID, successorErr := service.repository.SuccessorID(annotation.ID)
		if successorErr != nil {
			return nil, dto.PageMeta{}, Internal("could not resolve annotation successors", successorErr)
		}
		response.SupersededByID = successorID
		response.CurrentVersion = successorID == nil && annotation.AnnotationState != constants.AnnotationSuperseded
		responses = append(responses, response)
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
	return annotationResponse(after), nil
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
		annotations := service.repository.WithDB(tx)
		if transitionErr := annotations.Transition(id, before.AnnotationState, request.TargetState); transitionErr != nil {
			return Conflict("state_conflict", "annotation state changed concurrently", transitionErr)
		}
		after := before
		after.AnnotationState = request.TargetState
		summary := map[string]any{"reason_present": strings.TrimSpace(request.Reason) != ""}
		// When a replacement revision is locked it officially takes over from
		// the predecessor. The old result becomes read-only history and can no
		// longer participate in comparisons or adjudications.
		if request.TargetState == constants.AnnotationLocked && before.SupersedesID != nil {
			predecessor, loadErr := annotations.Get(*before.SupersedesID)
			if loadErr != nil {
				return Internal("could not load superseded annotation", loadErr)
			}
			if predecessor.AnnotationState != constants.AnnotationSuperseded {
				expected := predecessor.AnnotationState
				if transitionErr := annotations.Transition(predecessor.ID, expected, constants.AnnotationSuperseded); transitionErr != nil {
					return Conflict("state_conflict", "predecessor annotation changed concurrently", transitionErr)
				}
			}
			summary["superseded_predecessor_id"] = *before.SupersedesID
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "annotation_set."+request.TargetState, "annotation_set", auditID(id),
			summary, annotationSummary(before, labelCount(before.LabelsJSON)), annotationSummary(after, labelCount(before.LabelsJSON)))
	})
	if err != nil {
		return dto.AnnotationSetResponse{}, err
	}
	return service.Get(id)
}

func authorizeAnnotationTransition(annotation model.AnnotationSet, target string, actor dto.Actor) error {
	ownerAction := target == constants.AnnotationSubmitted || target == constants.AnnotationDraft
	if ownerAction && annotation.AnnotatorID != actor.ID {
		return Forbidden("annotators can submit or resume only their own annotation")
	}
	if target == constants.AnnotationLocked || target == constants.AnnotationReturned || target == constants.AnnotationCompared || target == constants.AnnotationSuperseded {
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
		SubmittedAt: annotation.SubmittedAt, SupersedesID: annotation.SupersedesID,
		ReplacementReason: annotation.ReplacementReason, QualityNote: annotation.QualityNote,
		CreatedAt: annotation.CreatedAt, UpdatedAt: annotation.UpdatedAt,
	}
}

func labelCount(encoded string) int {
	labels := []dto.AnnotationLabel{}
	_ = json.Unmarshal([]byte(encoded), &labels)
	return len(labels)
}

func annotationSummary(annotation model.AnnotationSet, labels int) map[string]any {
	return map[string]any{
		"dataset_id": annotation.DatasetID, "schema_id": annotation.SchemaID, "annotator_id": annotation.AnnotatorID,
		"item_key": annotation.ItemKey, "annotation_state": annotation.AnnotationState,
		"label_count": labels, "source_checksum": annotation.SourceChecksum,
	}
}
