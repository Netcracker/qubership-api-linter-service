package controller

import (
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"net/http"
)

type ValidationResultController interface {
	GetValidationSummaryForVersion(w http.ResponseWriter, r *http.Request)
	GetValidatedDocumentsForVersion(w http.ResponseWriter, r *http.Request)
	GetValidationResultForDocument(w http.ResponseWriter, r *http.Request)
}

func NewValidationResultController(validationService service.ValidationService) ValidationResultController {
	return &validationResultControllerImpl{
		validationService: validationService,
	}
}

type validationResultControllerImpl struct {
	validationService service.ValidationService
}

func (v validationResultControllerImpl) GetValidationSummaryForVersion(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")
	// FIXME: authorization check!
	/*ctx := secctx.Create(r)
	sufficientPrivileges, err := v.roleService.HasRequiredPermissions(ctx, packageId, view.ReadPermission)
	if err != nil {
		handlePkgRedirectOrRespondWithError(w, r, o.ptHandler, packageId, "Failed to check user privileges", err)
		return
	}
	if !sufficientPrivileges {
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusForbidden,
			Code:    exception.InsufficientPrivileges,
			Message: exception.InsufficientPrivilegesMsg,
		})
		return
	}*/
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

	result, err := v.validationService.GetVersionSummary(secctx.MakeUserContext(r), packageId, versionName)
	if err != nil {
		respondWithError(w, "Failed to get version summary", err)
	}
	if result == nil {
		//TODO: how to handle?

		// TODO: 404 or status==notValidated ????????

		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    "1234",           // TODO
			Message: "lint not found", // TODO
			Params:  nil,
			Debug:   "",
		})
	}
	respondWithJson(w, http.StatusOK, result)
}

func (v validationResultControllerImpl) GetValidatedDocumentsForVersion(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")
	// FIXME: authorization check!
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

	result, err := v.validationService.GetValidatedDocuments(secctx.MakeUserContext(r), packageId, versionName)
	if err != nil {
		respondWithError(w, "Failed to get validated documents for version", err)
		return
	}
	if result == nil {
		//TODO: how to handle? Err from service?
		// TODO: 404 or status==notValidated ????????
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    "1234",           // TODO
			Message: "lint not found", // TODO
			Params:  nil,
			Debug:   "",
		})
	}
	respondWithJson(w, http.StatusOK, result)
}

func (v validationResultControllerImpl) GetValidationResultForDocument(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")
	// FIXME: authorization check!
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
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	result, err := v.validationService.GetValidationResult(secctx.MakeUserContext(r), packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get validation result for document", err)
	}
	if result == nil {
		//TODO: how to handle?
		// TODO: 404 or status==notValidated ????????
		RespondWithCustomError(w, &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    "1234",           // TODO
			Message: "lint not found", // TODO
			Params:  nil,
			Debug:   "",
		})
	}
	respondWithJson(w, http.StatusOK, result)
}
