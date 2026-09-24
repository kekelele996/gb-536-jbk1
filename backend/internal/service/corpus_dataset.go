package service

import (
	"errors"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/model"
	"corpus-annotation-agreement-control/backend/internal/repository"
)

var datasetCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{2,63}$`)

type CorpusDatasetService struct {
	db         *gorm.DB
	repository *repository.CorpusDatasetRepository
	system     *SystemService
}

func NewCorpusDatasetService(db *gorm.DB, repository *repository.CorpusDatasetRepository, system *SystemService) *CorpusDatasetService {
	return &CorpusDatasetService{db: db, repository: repository, system: system}
}

func (service *CorpusDatasetService) Create(request dto.CreateCorpusDatasetRequest, actor dto.Actor, requestID string) (dto.CorpusDatasetResponse, error) {
	code := strings.ToUpper(strings.TrimSpace(request.DatasetCode))
	if !datasetCodePattern.MatchString(code) {
		return dto.CorpusDatasetResponse{}, Unprocessable("invalid_dataset_code", "dataset_code may contain uppercase letters, numbers, underscore, and hyphen", nil)
	}
	dataset := model.CorpusDataset{
		DatasetCode: code, Name: strings.TrimSpace(request.Name), Language: strings.ToLower(strings.TrimSpace(request.Language)),
		Domain: strings.TrimSpace(request.Domain), DocumentCount: request.DocumentCount,
		ContentMaskPolicy: strings.TrimSpace(request.ContentMaskPolicy), DatasetState: constants.DatasetDraft,
		Version: 1, OwnerTeam: strings.TrimSpace(request.OwnerTeam), CreatedBy: actor.ID,
	}
	err := service.db.Transaction(func(tx *gorm.DB) error {
		if createErr := service.repository.WithDB(tx).Create(&dataset); createErr != nil {
			if repository.IsUniqueViolation(createErr) {
				return Conflict("duplicate_dataset_code", "dataset_code already exists", createErr)
			}
			return Internal("could not create corpus dataset", createErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "corpus_dataset.created", "corpus_dataset", auditID(dataset.ID),
			map[string]any{"dataset_code": dataset.DatasetCode}, nil, datasetSummary(dataset))
	})
	if err != nil {
		return dto.CorpusDatasetResponse{}, err
	}
	return service.Get(dataset.ID)
}

func (service *CorpusDatasetService) Get(id uint) (dto.CorpusDatasetResponse, error) {
	dataset, err := service.repository.Get(id)
	if err != nil {
		return dto.CorpusDatasetResponse{}, MapRepositoryError("corpus dataset", err)
	}
	return service.response(dataset)
}

func (service *CorpusDatasetService) List(page, pageSize int, state, language, owner string) ([]dto.CorpusDatasetResponse, dto.PageMeta, error) {
	datasets, total, err := service.repository.List(page, pageSize, state, language, owner)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list corpus datasets", err)
	}
	responses := make([]dto.CorpusDatasetResponse, 0, len(datasets))
	for _, dataset := range datasets {
		response, err := service.response(dataset)
		if err != nil {
			return nil, dto.PageMeta{}, err
		}
		responses = append(responses, response)
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *CorpusDatasetService) Update(id uint, request dto.UpdateCorpusDatasetRequest, actor dto.Actor, requestID string) (dto.CorpusDatasetResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.CorpusDatasetResponse{}, MapRepositoryError("corpus dataset", err)
	}
	updated := before
	updated.Name = strings.TrimSpace(request.Name)
	updated.Language = strings.ToLower(strings.TrimSpace(request.Language))
	updated.Domain = strings.TrimSpace(request.Domain)
	updated.DocumentCount = request.DocumentCount
	updated.ContentMaskPolicy = strings.TrimSpace(request.ContentMaskPolicy)
	updated.OwnerTeam = strings.TrimSpace(request.OwnerTeam)
	var after model.CorpusDataset
	err = service.db.Transaction(func(tx *gorm.DB) error {
		datasets := service.repository.WithDB(tx)
		if updateErr := datasets.Update(&updated, request.Version); updateErr != nil {
			if errors.Is(updateErr, repository.ErrVersionConflict) {
				return Conflict("version_conflict", "dataset version changed or the dataset is no longer draft", updateErr)
			}
			return Internal("could not update corpus dataset", updateErr)
		}
		var reloadErr error
		after, reloadErr = datasets.Get(id)
		if reloadErr != nil {
			return Internal("could not reload corpus dataset", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "corpus_dataset.updated", "corpus_dataset", auditID(id),
			map[string]any{"expected_version": request.Version}, datasetSummary(before), datasetSummary(after))
	})
	if err != nil {
		return dto.CorpusDatasetResponse{}, err
	}
	return service.response(after)
}

func (service *CorpusDatasetService) Transition(id uint, target string, expectedVersion int, actor dto.Actor, requestID string) (dto.CorpusDatasetResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.CorpusDatasetResponse{}, MapRepositoryError("corpus dataset", err)
	}
	if !constants.CanTransitionDataset(before.DatasetState, target) {
		return dto.CorpusDatasetResponse{}, Conflict("invalid_dataset_transition",
			"dataset transition is not allowed from "+before.DatasetState+" to "+target, repository.ErrStateConflict)
	}
	var after model.CorpusDataset
	err = service.db.Transaction(func(tx *gorm.DB) error {
		datasets := service.repository.WithDB(tx)
		if transitionErr := datasets.Transition(id, expectedVersion, before.DatasetState, target); transitionErr != nil {
			return Conflict("state_conflict", "dataset state or version changed concurrently", transitionErr)
		}
		var reloadErr error
		after, reloadErr = datasets.Get(id)
		if reloadErr != nil {
			return Internal("could not reload corpus dataset", reloadErr)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "corpus_dataset."+target, "corpus_dataset", auditID(id),
			map[string]any{"expected_version": expectedVersion}, datasetSummary(before), datasetSummary(after))
	})
	if err != nil {
		return dto.CorpusDatasetResponse{}, err
	}
	return service.response(after)
}

func (service *CorpusDatasetService) response(dataset model.CorpusDataset) (dto.CorpusDatasetResponse, error) {
	schemas, annotations, err := service.repository.Counts(dataset.ID)
	if err != nil {
		return dto.CorpusDatasetResponse{}, Internal("could not summarize corpus dataset", err)
	}
	return dto.CorpusDatasetResponse{
		ID: dataset.ID, DatasetCode: dataset.DatasetCode, Name: dataset.Name, Language: dataset.Language,
		Domain: dataset.Domain, DocumentCount: dataset.DocumentCount, ContentMaskPolicy: dataset.ContentMaskPolicy,
		DatasetState: dataset.DatasetState, Version: dataset.Version, OwnerTeam: dataset.OwnerTeam,
		SchemaCount: schemas, AnnotationCount: annotations, CreatedAt: dataset.CreatedAt, UpdatedAt: dataset.UpdatedAt,
	}, nil
}

func datasetSummary(dataset model.CorpusDataset) map[string]any {
	return map[string]any{
		"dataset_code": dataset.DatasetCode, "dataset_state": dataset.DatasetState, "version": dataset.Version,
		"language": dataset.Language, "domain": dataset.Domain, "document_count": dataset.DocumentCount,
		"owner_team": dataset.OwnerTeam, "mask_policy_configured": dataset.ContentMaskPolicy != "",
	}
}
