package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/service"
)

type CorpusDatasetHandler struct{ service *service.CorpusDatasetService }

func NewCorpusDatasetHandler(service *service.CorpusDatasetService) *CorpusDatasetHandler {
	return &CorpusDatasetHandler{service: service}
}

func (handler *CorpusDatasetHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	items, meta, err := handler.service.List(
		page, pageSize, context.Query("state"), context.Query("language"), context.Query("owner_team"),
	)
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, items, meta)
}

func (handler *CorpusDatasetHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}

func (handler *CorpusDatasetHandler) Create(context *gin.Context) {
	var request dto.CreateCorpusDatasetRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Create(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusCreated, item)
}

func (handler *CorpusDatasetHandler) Update(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.UpdateCorpusDatasetRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Update(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}

func (handler *CorpusDatasetHandler) Transition(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request struct {
		TargetState string `json:"target_state" validate:"required,oneof=frozen archived"`
		Version     int    `json:"version" validate:"required,gte=1"`
	}
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Transition(id, request.TargetState, request.Version, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}
