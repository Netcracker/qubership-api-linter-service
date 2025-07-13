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

package service

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/google/uuid"
	"net/http"
	"time"
)

type ValidationService interface {
	ValidateVersion(packageId string, version string, revision int, eventId string) (string, error)

	// TODO: get status, result, etc
	ValidateFiles(engine string, files []string) (interface{}, error) // TODO: remove
}

func NewValidationService(repo repository.VersionLintTaskRepository, versionTaskProcessor VersionTaskProcessor, executorId string) ValidationService {
	return &validationServiceImpl{
		repo:                 repo,
		versionTaskProcessor: versionTaskProcessor,
		executorId:           executorId,
	}
}

type validationServiceImpl struct {
	repo                 repository.VersionLintTaskRepository
	versionTaskProcessor VersionTaskProcessor
	executorId           string
}

func (v validationServiceImpl) ValidateVersion(packageId string, version string, revision int, eventId string) (string, error) {
	ent := entity.VersionLintTask{
		Id:           uuid.NewString(),
		PackageId:    packageId,
		Version:      version,
		Revision:     revision,
		Status:       "none", // TODO: const
		Details:      "",
		CreatedAt:    time.Now(),
		ExecutorId:   v.executorId, // reserve the task for current instance to start processing immediately
		LastActive:   time.Now(),
		EventId:      eventId, // optional
		RestartCount: 0,
		Priority:     0,
	}
	err := v.repo.SaveVersionTask(context.Background(), ent)
	if err != nil {
		return "", err
	}

	err = v.versionTaskProcessor.StartVersionLintTask(ent.Id)
	if err != nil {
		return "", err
	}

	return ent.Id, nil
}

const tempFolder = "tmp"

// TODO: remove
func (v validationServiceImpl) ValidateFiles(engine string, files []string) (interface{}, error) {
	var err error
	var report interface{}
	switch engine {
	case "vacuum":
		report, err = v.runDocumentsVacuum(files)
	case "spectral":
		report, err = v.runDocumentsSpectral(files)
	default:
		err = &exception.CustomError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("Value %s is incorrect validation engine. Available options are: vacuum, spectral.", engine),
		}
	}
	return report, err
}
