package controller

import (
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"net/http"
)

type ScoringController interface {
	GetScoringData(w http.ResponseWriter, r *http.Request)
	GetEnhancedScoreData(w http.ResponseWriter, r *http.Request)

	RunScoring(w http.ResponseWriter, r *http.Request)
	GetScoringStatus(w http.ResponseWriter, r *http.Request)
}

func NewScoringController(scoringService service.ScoringService,
	authorizationService service.AuthorizationService) ScoringController {
	return &scoringControllerImpl{
		scoringService:       scoringService,
		authorizationService: authorizationService,
	}
}

type scoringControllerImpl struct {
	scoringService       service.ScoringService
	authorizationService service.AuthorizationService
}

func (s scoringControllerImpl) RunScoring(w http.ResponseWriter, r *http.Request) {

	/*packageId := getStringParam(r, "packageId")

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
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	s.scoringService.MakeRestDocScore()*/

}

func (s scoringControllerImpl) GetScoringStatus(w http.ResponseWriter, r *http.Request) {
	//TODO implement me
	panic("implement me")
}

func (s scoringControllerImpl) GetScoringData(w http.ResponseWriter, r *http.Request) {
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
			Params:  map[string]interface{}{"param": "version"},
			Debug:   err.Error(),
		})
		return
	}

	result, err := s.scoringService.GetRestDocScoringData(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get scoring data", err)
		return
	}
	respondWithJson(w, http.StatusOK, result)
}

func (s scoringControllerImpl) GetEnhancedScoreData(w http.ResponseWriter, r *http.Request) {
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

	result, err := s.scoringService.GetEnhancedRestDocScoringData(ctx, packageId, versionName, slug)
	if err != nil {
		respondWithError(w, "Failed to get enhanced scoring data", err)
		return
	}
	respondWithJson(w, http.StatusOK, result)
}
