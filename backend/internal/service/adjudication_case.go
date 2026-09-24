package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/algorithm"
	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/matching"
	"corpus-annotation-agreement-control/backend/internal/model"
	"corpus-annotation-agreement-control/backend/internal/repository"
)

type AdjudicationCaseService struct {
	db               *gorm.DB
	repository       *repository.AdjudicationCaseRepository
	annotations      *repository.AnnotationSetRepository
	system           *SystemService
	algorithmVersion string
}

func NewAdjudicationCaseService(db *gorm.DB, repository *repository.AdjudicationCaseRepository, annotations *repository.AnnotationSetRepository, system *SystemService, algorithmVersion string) *AdjudicationCaseService {
	return &AdjudicationCaseService{
		db: db, repository: repository, annotations: annotations,
		system: system, algorithmVersion: algorithmVersion,
	}
}

func (service *AdjudicationCaseService) Compute(request dto.ComputeAdjudicationRequest, idempotencyKey string, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return dto.AdjudicationCaseResponse{}, false, BadRequest("missing_idempotency_key", "Idempotency-Key header is required")
	}
	annotations, err := service.annotations.ByIDs(request.AnnotationSetIDs)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, false, MapRepositoryError("annotation sets", err)
	}
	if len(annotations) != len(uniqueIDs(request.AnnotationSetIDs)) {
		return dto.AdjudicationCaseResponse{}, false, Unprocessable("annotation_set_mismatch", "all requested annotation sets must exist exactly once", nil)
	}
	labelsBySet, ratingSets, schema, err := validateComparableAnnotations(annotations, request)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, false, err
	}
	inputHash := adjudicationInputHash(annotations, request, service.algorithmVersion)
	if existing, findErr := service.repository.FindByIdempotencyKey(idempotencyKey); findErr == nil {
		if existing.InputHash != inputHash {
			return dto.AdjudicationCaseResponse{}, false, Conflict("idempotency_conflict", "Idempotency-Key was already used for different inputs", repository.ErrStateConflict)
		}
		response := adjudicationResponse(existing)
		response.Reused = true
		return response, true, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return dto.AdjudicationCaseResponse{}, false, Internal("could not check computation idempotency", findErr)
	}
	if existing, findErr := service.repository.LatestByInput(inputHash, service.algorithmVersion); findErr == nil {
		response := adjudicationResponse(existing)
		response.Reused = true
		return response, true, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return dto.AdjudicationCaseResponse{}, false, Internal("could not check prior agreement computation", findErr)
	}
	agreement, err := algorithm.Compute(request.Metric, ratingSets)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, false, Unprocessable("agreement_not_applicable", err.Error(), err)
	}
	evidence, confusion := compareAll(labelsBySet)
	if len(evidence) == 0 {
		return dto.AdjudicationCaseResponse{}, false, Unprocessable("no_disagreement", "annotation sets agree and do not require adjudication", nil)
	}
	disagreementType, labelPair := summarizeEvidence(evidence)
	annotationIDsJSON, _ := json.Marshal(sortedIDs(request.AnnotationSetIDs))
	confusionJSON, _ := json.Marshal(confusion)
	evidenceJSON, _ := json.Marshal(evidence)
	caseRecord := model.AdjudicationCase{
		DatasetID: request.DatasetID, ItemKey: strings.TrimSpace(request.ItemKey),
		AnnotationSetIDsJSON: string(annotationIDsJSON), AgreementMetric: agreement.Metric,
		AgreementScore: agreement.Score, ObservedAgreement: agreement.ObservedAgreement,
		ChanceAgreement: agreement.ChanceAgreement, SampleSize: agreement.SampleSize,
		CoderCount: agreement.CoderCount, MissingValueCount: agreement.MissingValueCount,
		Applicability: agreement.Applicability, DisagreementType: disagreementType,
		ConfusionSnapshotJSON: string(confusionJSON), EvidenceSnapshotJSON: string(evidenceJSON),
		ClusterKey: matching.ClusterKey(schema.SchemaCode, disagreementType, labelPair),
		CaseState:  constants.CaseOpen, FinalLabelsJSON: "[]", Rationale: "",
		InputHash: inputHash, AlgorithmVersion: service.algorithmVersion,
		IdempotencyKey: idempotencyKey, CreatedBy: actor.ID,
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		cases := service.repository.WithDB(tx)
		annotationRepository := service.annotations.WithDB(tx)
		if createErr := cases.Create(&caseRecord); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("idempotency_conflict", "agreement computation was already created", createErr)
			}
			return Internal("could not create adjudication case", createErr)
		}
		for _, annotation := range annotations {
			if annotation.AnnotationState == constants.AnnotationLocked {
				if transitionErr := annotationRepository.Transition(annotation.ID, constants.AnnotationLocked, constants.AnnotationCompared); transitionErr != nil {
					return Conflict("state_conflict", "annotation changed while agreement was computed", transitionErr)
				}
			}
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case.computed", "adjudication_case", auditID(caseRecord.ID),
			map[string]any{"annotation_set_ids": sortedIDs(request.AnnotationSetIDs), "input_hash": inputHash},
			nil, caseSummary(caseRecord))
	})
	if err != nil {
		if repository.IsUniqueViolation(err) {
			existing, findErr := service.repository.FindByIdempotencyKey(idempotencyKey)
			if findErr == nil && existing.InputHash == inputHash {
				response := adjudicationResponse(existing)
				response.Reused = true
				return response, true, nil
			}
		}
		return dto.AdjudicationCaseResponse{}, false, err
	}
	created, err := service.Get(caseRecord.ID)
	return created, false, err
}

func (service *AdjudicationCaseService) Get(id uint) (dto.AdjudicationCaseResponse, error) {
	adjudication, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, MapRepositoryError("adjudication case", err)
	}
	return adjudicationResponse(adjudication), nil
}

func (service *AdjudicationCaseService) List(page, pageSize int, datasetID uint, state, disagreementType, clusterKey string) ([]dto.AdjudicationCaseResponse, dto.PageMeta, error) {
	cases, total, err := service.repository.List(page, pageSize, datasetID, state, disagreementType, clusterKey)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list adjudication cases", err)
	}
	responses := make([]dto.AdjudicationCaseResponse, 0, len(cases))
	for _, adjudication := range cases {
		responses = append(responses, adjudicationResponse(adjudication))
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *AdjudicationCaseService) Assign(id uint, request dto.AssignCaseRequest, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, MapRepositoryError("adjudication case", err)
	}
	if before.CaseState != constants.CaseOpen && before.CaseState != constants.CaseReopened {
		return dto.AdjudicationCaseResponse{}, Conflict("invalid_case_transition", "only open or reopened cases can be assigned", repository.ErrStateConflict)
	}
	ownsAnnotation, ownershipErr := ownsAnyAnnotation(service.annotations, before.AnnotationSetIDsJSON, actor.ID)
	if ownershipErr != nil {
		return dto.AdjudicationCaseResponse{}, Internal("case annotation ownership could not be verified", ownershipErr)
	}
	if ownsAnnotation {
		return dto.AdjudicationCaseResponse{}, Forbidden("adjudicators cannot claim a case containing their own annotation")
	}
	var after model.AdjudicationCase
	err = service.db.Transaction(func(tx *gorm.DB) error {
		cases := service.repository.WithDB(tx)
		if assignErr := cases.Assign(id, actor.ID); assignErr != nil {
			return Conflict("state_conflict", "case assignment changed concurrently", assignErr)
		}
		var reloadErr error
		after, reloadErr = cases.Get(id)
		if reloadErr != nil {
			return Internal("could not reload adjudication case", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case.assigned", "adjudication_case", auditID(id),
			map[string]any{"note_present": strings.TrimSpace(request.Note) != ""}, caseSummary(before), caseSummary(after))
	})
	if err != nil {
		return dto.AdjudicationCaseResponse{}, err
	}
	return adjudicationResponse(after), nil
}

func (service *AdjudicationCaseService) Decide(id uint, request dto.AdjudicateCaseRequest, idempotencyKey string, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return dto.AdjudicationCaseResponse{}, false, BadRequest("missing_idempotency_key", "Idempotency-Key header is required for adjudication")
	}
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, false, MapRepositoryError("adjudication case", err)
	}
	if before.DecisionIdempotencyKey != nil {
		if *before.DecisionIdempotencyKey == idempotencyKey {
			response := adjudicationResponse(before)
			response.Reused = true
			return response, true, nil
		}
		return dto.AdjudicationCaseResponse{}, false, Conflict("idempotency_conflict", "case was already adjudicated with a different Idempotency-Key", repository.ErrStateConflict)
	}
	if before.AdjudicatorID == nil || *before.AdjudicatorID != actor.ID {
		return dto.AdjudicationCaseResponse{}, false, Forbidden("only the assigned adjudicator can decide this case")
	}
	ownsAnnotation, ownershipErr := ownsAnyAnnotation(service.annotations, before.AnnotationSetIDsJSON, actor.ID)
	if ownershipErr != nil {
		return dto.AdjudicationCaseResponse{}, false, Internal("case annotation ownership could not be verified", ownershipErr)
	}
	if ownsAnnotation {
		return dto.AdjudicationCaseResponse{}, false, Forbidden("adjudicators cannot decide a case containing their own annotation")
	}
	annotations, snapshotErr := loadCaseAnnotations(service.annotations, before.AnnotationSetIDsJSON)
	if snapshotErr != nil {
		return dto.AdjudicationCaseResponse{}, false, Internal("case annotation snapshot could not be resolved", snapshotErr)
	}
	labels, err := normalizeAndValidateLabels(request.FinalLabels, annotations[0].Schema)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, false, err
	}
	labelsJSON, _ := json.Marshal(labels)
	err = service.db.Transaction(func(tx *gorm.DB) error {
		if decideErr := service.repository.WithDB(tx).Decide(id, actor.ID, string(labelsJSON), strings.TrimSpace(request.Rationale), idempotencyKey); decideErr != nil {
			return Conflict("state_conflict", "case decision changed concurrently", decideErr)
		}
		after := before
		after.CaseState = constants.CaseAdjudicated
		return service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case.adjudicated", "adjudication_case", auditID(id),
			map[string]any{"label_count": len(labels), "decision_idempotency_key": idempotencyKey},
			caseSummary(before), caseSummary(after))
	})
	if err != nil {
		current, reloadErr := service.repository.Get(id)
		if reloadErr == nil && current.DecisionIdempotencyKey != nil && *current.DecisionIdempotencyKey == idempotencyKey {
			response := adjudicationResponse(current)
			response.Reused = true
			return response, true, nil
		}
		return dto.AdjudicationCaseResponse{}, false, err
	}
	response, err := service.Get(id)
	return response, false, err
}

func (service *AdjudicationCaseService) Review(id uint, request dto.CaseReviewRequest, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, MapRepositoryError("adjudication case", err)
	}
	if before.AdjudicatorID != nil && *before.AdjudicatorID == actor.ID {
		return dto.AdjudicationCaseResponse{}, Forbidden("the deciding adjudicator cannot independently review the same case")
	}
	ownsAnnotation, ownershipErr := ownsAnyAnnotation(service.annotations, before.AnnotationSetIDsJSON, actor.ID)
	if ownershipErr != nil {
		return dto.AdjudicationCaseResponse{}, Internal("case annotation ownership could not be verified", ownershipErr)
	}
	if ownsAnnotation {
		return dto.AdjudicationCaseResponse{}, Forbidden("reviewers cannot review a case containing their own annotation")
	}
	return service.caseTransition(before, constants.CaseReviewed, request.Note, actor, requestID)
}

func (service *AdjudicationCaseService) Accept(id uint, request dto.CaseReviewRequest, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, MapRepositoryError("adjudication case", err)
	}
	if before.ReviewedBy == nil || *before.ReviewedBy != actor.ID {
		return dto.AdjudicationCaseResponse{}, Forbidden("only the independent reviewer can accept this case")
	}
	return service.caseTransition(before, constants.CaseAccepted, request.Note, actor, requestID)
}

func (service *AdjudicationCaseService) Reopen(id uint, request dto.CaseReviewRequest, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.AdjudicationCaseResponse{}, MapRepositoryError("adjudication case", err)
	}
	if before.ReviewedBy == nil || *before.ReviewedBy != actor.ID {
		return dto.AdjudicationCaseResponse{}, Forbidden("only the independent reviewer can reopen this case")
	}
	return service.caseTransition(before, constants.CaseReopened, request.Note, actor, requestID)
}

func (service *AdjudicationCaseService) caseTransition(before model.AdjudicationCase, target, note string, actor dto.Actor, requestID string) (dto.AdjudicationCaseResponse, error) {
	if !constants.CanTransitionCase(before.CaseState, target) {
		return dto.AdjudicationCaseResponse{}, Conflict("invalid_case_transition",
			"case transition is not allowed from "+before.CaseState+" to "+target, repository.ErrStateConflict)
	}
	var after model.AdjudicationCase
	err := service.db.Transaction(func(tx *gorm.DB) error {
		cases := service.repository.WithDB(tx)
		if reviewErr := cases.Review(before.ID, before.CaseState, target, actor.ID, strings.TrimSpace(note)); reviewErr != nil {
			return Conflict("state_conflict", "case state changed concurrently", reviewErr)
		}
		var reloadErr error
		after, reloadErr = cases.Get(before.ID)
		if reloadErr != nil {
			return Internal("could not reload adjudication case", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "adjudication_case."+target, "adjudication_case", auditID(before.ID),
			map[string]any{"review_note_present": strings.TrimSpace(note) != ""}, caseSummary(before), caseSummary(after))
	})
	if err != nil {
		return dto.AdjudicationCaseResponse{}, err
	}
	return adjudicationResponse(after), nil
}

func validateComparableAnnotations(annotations []model.AnnotationSet, request dto.ComputeAdjudicationRequest) ([][]dto.AnnotationLabel, []algorithm.RatingSet, model.AnnotationSchema, error) {
	if len(annotations) < 2 {
		return nil, nil, model.AnnotationSchema{}, Unprocessable("insufficient_annotations", "at least two annotation sets are required", nil)
	}
	annotators := map[uint]bool{}
	labelsBySet := make([][]dto.AnnotationLabel, 0, len(annotations))
	ratingSets := make([]algorithm.RatingSet, 0, len(annotations))
	var schema model.AnnotationSchema
	for index, annotation := range annotations {
		if annotation.DatasetID != request.DatasetID || annotation.ItemKey != request.ItemKey {
			return nil, nil, model.AnnotationSchema{}, Unprocessable("annotation_set_mismatch", "all annotation sets must belong to the requested dataset item", nil)
		}
		if index == 0 {
			schema = annotation.Schema
		} else if annotation.SchemaID != schema.ID {
			return nil, nil, model.AnnotationSchema{}, Unprocessable("incompatible_schema", "all annotation sets must use the same schema version", nil)
		}
		if annotation.AnnotationState != constants.AnnotationLocked && annotation.AnnotationState != constants.AnnotationCompared {
			return nil, nil, model.AnnotationSchema{}, Conflict("annotation_not_locked", "annotation sets must be locked before comparison", repository.ErrStateConflict)
		}
		annotators[annotation.AnnotatorID] = true
		labels := []dto.AnnotationLabel{}
		if err := json.Unmarshal([]byte(annotation.LabelsJSON), &labels); err != nil {
			return nil, nil, model.AnnotationSchema{}, Internal("annotation labels could not be decoded", err)
		}
		labelsBySet = append(labelsBySet, labels)
		ratingSets = append(ratingSets, ratingSet(annotation.Annotator.Username, labels))
	}
	if len(annotators) < 2 {
		return nil, nil, model.AnnotationSchema{}, Unprocessable("insufficient_annotators", "comparison requires annotations from at least two distinct annotators", nil)
	}
	return labelsBySet, ratingSets, schema, nil
}

func ratingSet(coder string, labels []dto.AnnotationLabel) algorithm.RatingSet {
	ratings := map[string]string{}
	for _, label := range labels {
		key := label.UnitKey
		if label.IsSpan() {
			key = fmt.Sprintf("%s:%d-%d", label.UnitKey, label.Start, label.End)
		}
		ratings[key] = label.Label
	}
	return algorithm.RatingSet{Coder: coder, Ratings: ratings}
}

func compareAll(labelsBySet [][]dto.AnnotationLabel) ([]dto.DiffEvidence, []dto.ConfusionCell) {
	allEvidence := make([]dto.DiffEvidence, 0)
	counts := map[string]int{}
	for index := 1; index < len(labelsBySet); index++ {
		evidence, confusion, _, _ := matching.CompareAnnotations(labelsBySet[0], labelsBySet[index])
		allEvidence = append(allEvidence, evidence...)
		for _, cell := range confusion {
			counts[cell.LeftLabel+"\x00"+cell.RightLabel] += cell.Count
		}
	}
	confusion := make([]dto.ConfusionCell, 0, len(counts))
	for key, count := range counts {
		parts := strings.SplitN(key, "\x00", 2)
		confusion = append(confusion, dto.ConfusionCell{LeftLabel: parts[0], RightLabel: parts[1], Count: count})
	}
	sort.Slice(confusion, func(left, right int) bool {
		if confusion[left].LeftLabel == confusion[right].LeftLabel {
			return confusion[left].RightLabel < confusion[right].RightLabel
		}
		return confusion[left].LeftLabel < confusion[right].LeftLabel
	})
	return allEvidence, confusion
}

func summarizeEvidence(evidence []dto.DiffEvidence) (string, string) {
	counts := map[string]int{}
	pairs := map[string]map[string]int{}
	for _, item := range evidence {
		counts[item.Type]++
		left, right := item.LeftLabel, item.RightLabel
		if left == "" {
			left = "∅"
		}
		if right == "" {
			right = "∅"
		}
		if pairs[item.Type] == nil {
			pairs[item.Type] = map[string]int{}
		}
		pairs[item.Type][left+"~"+right]++
	}
	priority := []string{constants.DisagreementOverlap, constants.DisagreementOmission, constants.DisagreementLabel, constants.DisagreementBoundary}
	selected, highest := constants.DisagreementLabel, -1
	for _, kind := range priority {
		if counts[kind] > highest {
			selected, highest = kind, counts[kind]
		}
	}
	pair, pairCount := "", -1
	for candidate, count := range pairs[selected] {
		if count > pairCount || (count == pairCount && candidate < pair) {
			pair, pairCount = candidate, count
		}
	}
	return selected, pair
}

func adjudicationInputHash(annotations []model.AnnotationSet, request dto.ComputeAdjudicationRequest, version string) string {
	parts := []string{fmt.Sprintf("%d", request.DatasetID), request.ItemKey, request.Metric, version}
	sort.Slice(annotations, func(left, right int) bool { return annotations[left].ID < annotations[right].ID })
	for _, annotation := range annotations {
		parts = append(parts, fmt.Sprintf("%d:%s:%s", annotation.ID, annotation.SourceChecksum, annotation.LabelsJSON))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func adjudicationResponse(adjudication model.AdjudicationCase) dto.AdjudicationCaseResponse {
	ids := []uint{}
	confusion := []dto.ConfusionCell{}
	evidence := []dto.DiffEvidence{}
	finalLabels := []dto.AnnotationLabel{}
	_ = json.Unmarshal([]byte(adjudication.AnnotationSetIDsJSON), &ids)
	_ = json.Unmarshal([]byte(adjudication.ConfusionSnapshotJSON), &confusion)
	_ = json.Unmarshal([]byte(adjudication.EvidenceSnapshotJSON), &evidence)
	_ = json.Unmarshal([]byte(adjudication.FinalLabelsJSON), &finalLabels)
	return dto.AdjudicationCaseResponse{
		ID: adjudication.ID, DatasetID: adjudication.DatasetID, DatasetCode: adjudication.Dataset.DatasetCode,
		ItemKey: adjudication.ItemKey, AnnotationSetIDs: ids,
		Agreement: dto.AgreementDetails{
			Metric: adjudication.AgreementMetric, Score: adjudication.AgreementScore,
			ObservedAgreement: adjudication.ObservedAgreement, ChanceAgreement: adjudication.ChanceAgreement,
			SampleSize: adjudication.SampleSize, CoderCount: adjudication.CoderCount,
			MissingValueCount: adjudication.MissingValueCount, Applicability: adjudication.Applicability,
		},
		DisagreementType: adjudication.DisagreementType, ConfusionSnapshot: confusion,
		EvidenceSnapshot: evidence, ClusterKey: adjudication.ClusterKey, CaseState: adjudication.CaseState,
		FinalLabels: finalLabels, Rationale: adjudication.Rationale, AdjudicatorID: adjudication.AdjudicatorID,
		ReviewedBy: adjudication.ReviewedBy, DecidedAt: adjudication.DecidedAt, InputHash: adjudication.InputHash,
		AlgorithmVersion: adjudication.AlgorithmVersion, IdempotencyKey: adjudication.IdempotencyKey,
		DecisionIdempotencyKey: adjudication.DecisionIdempotencyKey, ReopenCount: adjudication.ReopenCount,
		CreatedBy: adjudication.CreatedBy, CreatedAt: adjudication.CreatedAt, UpdatedAt: adjudication.UpdatedAt,
	}
}

func ownsAnyAnnotation(repository *repository.AnnotationSetRepository, encodedIDs string, actorID uint) (bool, error) {
	annotations, err := loadCaseAnnotations(repository, encodedIDs)
	if err != nil {
		return false, err
	}
	for _, annotation := range annotations {
		if annotation.AnnotatorID == actorID {
			return true, nil
		}
	}
	return false, nil
}

func loadCaseAnnotations(repository *repository.AnnotationSetRepository, encodedIDs string) ([]model.AnnotationSet, error) {
	ids := []uint{}
	if err := json.Unmarshal([]byte(encodedIDs), &ids); err != nil {
		return nil, fmt.Errorf("decode annotation snapshot ids: %w", err)
	}
	if len(ids) == 0 {
		return nil, errors.New("annotation snapshot contains no ids")
	}
	annotations, err := repository.ByIDs(ids)
	if err != nil {
		return nil, err
	}
	if len(annotations) != len(uniqueIDs(ids)) {
		return nil, errors.New("annotation snapshot is incomplete")
	}
	return annotations, nil
}

func caseSummary(adjudication model.AdjudicationCase) map[string]any {
	return map[string]any{
		"dataset_id": adjudication.DatasetID, "item_key": adjudication.ItemKey,
		"agreement_metric": adjudication.AgreementMetric, "agreement_score": adjudication.AgreementScore,
		"observed_agreement": adjudication.ObservedAgreement, "chance_agreement": adjudication.ChanceAgreement,
		"sample_size": adjudication.SampleSize, "disagreement_type": adjudication.DisagreementType,
		"cluster_key": adjudication.ClusterKey, "case_state": adjudication.CaseState,
		"input_hash": adjudication.InputHash, "algorithm_version": adjudication.AlgorithmVersion,
	}
}

func uniqueIDs(values []uint) map[uint]bool {
	result := map[uint]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sortedIDs(values []uint) []uint {
	result := append([]uint{}, values...)
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}
