package controller

import (
	"encoding/json"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"io"
	"net/http"
)

type EnhancementController interface {
	EnhanceDoc(w http.ResponseWriter, r *http.Request)
	GetStatus(w http.ResponseWriter, r *http.Request)
	GetEnhancedDoc(w http.ResponseWriter, r *http.Request)
	PublishEnhancedDocs(w http.ResponseWriter, r *http.Request)
}

func NewEnhancementController(enhancementService service.EnhancementService,
	authorizationService service.AuthorizationService) EnhancementController {
	return &enhancementControllerImpl{
		enhancementService:   enhancementService,
		authorizationService: authorizationService,
	}
}

type enhancementControllerImpl struct {
	enhancementService   service.EnhancementService
	authorizationService service.AuthorizationService
}

func (e enhancementControllerImpl) EnhanceDoc(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")

	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := e.authorizationService.HasPublishPackagePermission(ctx, packageId)
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

	versionName, err := getUnescapedStringParam(r, "version")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	slug, err := getUnescapedStringParam(r, "slug")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "slug"},
			Debug:   err.Error(),
		})
		return
	}

	err = e.enhancementService.EnhanceDoc(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to enhance document", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (e enhancementControllerImpl) GetStatus(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")

	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := e.authorizationService.HasReadPackagePermission(ctx, packageId)
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

	versionName, err := getUnescapedStringParam(r, "version")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	slug, err := getUnescapedStringParam(r, "slug")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "slug"},
			Debug:   err.Error(),
		})
		return
	}

	status, err := e.enhancementService.GetEnhanceStatus(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get enhancement status", err)
		return
	}
	respondWithJson(w, http.StatusOK, status)
}

func (e enhancementControllerImpl) GetEnhancedDoc(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")

	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := e.authorizationService.HasReadPackagePermission(ctx, packageId)
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

	versionName, err := getUnescapedStringParam(r, "version")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	slug, err := getUnescapedStringParam(r, "slug")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "slug"},
			Debug:   err.Error(),
		})
		return
	}

	content, err := e.enhancementService.GetEnhancedDoc(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get enhanced document", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

func (e enhancementControllerImpl) PublishEnhancedDocs(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")

	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := e.authorizationService.HasPublishPackagePermission(ctx, packageId)
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

	versionName, err := getUnescapedStringParam(r, "version")
	if err != nil {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.InvalidURLEscape,
			Message: exception.InvalidURLEscapeMsg,
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
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
	var req view.PublishEnhancementRequest
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

	response, err := e.enhancementService.PublishEnhancedDocs(ctx, packageId, versionName, req)
	if err != nil {
		respondWithError(w, "Failed to publish enhanced documents", err)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
