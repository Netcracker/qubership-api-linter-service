package controller

import (
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"net/http"
)

type ProblemsController interface {
	GetProblemsData(w http.ResponseWriter, r *http.Request)
}

func NewProblemsController(problemsService service.ProblemsService, authorizationService service.AuthorizationService) ProblemsController {
	return &problemsControllerImpl{docProblemsService: problemsService, authorizationService: authorizationService}
}

type problemsControllerImpl struct {
	docProblemsService   service.ProblemsService
	authorizationService service.AuthorizationService
}

func (s problemsControllerImpl) GetProblemsData(w http.ResponseWriter, r *http.Request) {
	packageId := getStringParam(r, "packageId")

	ctx := secctx.MakeUserContext(r)
	sufficientPrivileges, err := s.authorizationService.HasReadPackagePermission(ctx, packageId)
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

	result, err := s.docProblemsService.GetDocProblems(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get problems data", err)
		return
	}
	respondWithJson(w, http.StatusOK, result)
}
