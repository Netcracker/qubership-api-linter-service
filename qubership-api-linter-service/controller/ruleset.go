// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"net/http"
)

type RulesetController interface {
	GetRuleset(w http.ResponseWriter, r *http.Request)
}

type rulesetControllerImpl struct {
	rulesetService service.RulesetService
}

func NewRulesetController(rulesetService service.RulesetService) RulesetController {
	return &rulesetControllerImpl{rulesetService: rulesetService}
}

func (c rulesetControllerImpl) GetRuleset(w http.ResponseWriter, r *http.Request) {
	rulesetId := getStringParam(r, "ruleset_id")
	// FIXME: authorization check!
	result, err := c.rulesetService.GetRuleset(rulesetId)
	if err != nil {
		respondWithError(w, "Failed to get ruleset", err)
		return
	}
	if result == nil {
		//TODO: how to handle?
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
