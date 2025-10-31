package controller

import (
	"encoding/json"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"io"
	"net/http"
)

type LLMTuningController interface {
	UpdateGenerateProblemsPrompt(w http.ResponseWriter, r *http.Request)
	UpdateFixProblemsPrompt(w http.ResponseWriter, r *http.Request)
	UpdateModel(w http.ResponseWriter, r *http.Request)
}

func NewLLMTuningController(openaiClient client.LLMClient, authorizationService service.AuthorizationService) LLMTuningController {
	return &llmTuningControllerImpl{openaiClient: openaiClient, authorizationService: authorizationService}
}

type llmTuningControllerImpl struct {
	openaiClient         client.LLMClient
	authorizationService service.AuthorizationService
}

func (l llmTuningControllerImpl) UpdateGenerateProblemsPrompt(w http.ResponseWriter, r *http.Request) {
	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := l.authorizationService.HasRulesetManagementPermission(ctx) // TODO: admin check
	if err != nil {
		respondWithError(w, "Failed to check permissions", err)
		return
	}
	if !sufficientPrivileges {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusForbidden,
			Code:    exception.InsufficientPrivileges,
			Message: exception.InsufficientPrivilegesMsg,
		})
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}
	var req view.UpdatePromptReq
	err = json.Unmarshal(body, &req)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}

	l.openaiClient.UpdateGenerateProblemsPrompt(req.Prompt)
}

func (l llmTuningControllerImpl) UpdateFixProblemsPrompt(w http.ResponseWriter, r *http.Request) {
	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := l.authorizationService.HasRulesetManagementPermission(ctx) // TODO: admin check
	if err != nil {
		respondWithError(w, "Failed to check permissions", err)
		return
	}
	if !sufficientPrivileges {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusForbidden,
			Code:    exception.InsufficientPrivileges,
			Message: exception.InsufficientPrivilegesMsg,
		})
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}
	var req view.UpdatePromptReq
	err = json.Unmarshal(body, &req)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}

	l.openaiClient.UpdateFixProblemsPrompt(req.Prompt)
}

func (l llmTuningControllerImpl) UpdateModel(w http.ResponseWriter, r *http.Request) {
	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := l.authorizationService.HasRulesetManagementPermission(ctx) // TODO: admin check
	if err != nil {
		respondWithError(w, "Failed to check permissions", err)
		return
	}
	if !sufficientPrivileges {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusForbidden,
			Code:    exception.InsufficientPrivileges,
			Message: exception.InsufficientPrivilegesMsg,
		})
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}
	var req view.UpdateModelReq
	err = json.Unmarshal(body, &req)
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.BadRequestBody,
			Message: exception.BadRequestBodyMsg,
			Debug:   err.Error(),
		})
		return
	}

	err = l.openaiClient.UpdateModel(req.Model)
	if err != nil {
		respondWithError(w, "Failed to update model", err)
		return
	}
}
