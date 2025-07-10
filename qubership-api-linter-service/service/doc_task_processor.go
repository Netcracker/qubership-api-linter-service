package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DocTaskProcessor interface {
	Start()
}

func NewDocTaskProcessor(docTaskRepo repository.DocLintTaskRepository, ruleSetRepository repository.RuleSetRepository,
	docResultRepository repository.DocResultRepository, cl client.ApihubClient, spectralExecutor SpectralExecutor, executorId string) DocTaskProcessor {
	return &docTaskProcessorImpl{
		docTaskRepo:         docTaskRepo,
		ruleSetRepository:   ruleSetRepository,
		docResultRepository: docResultRepository,
		cl:                  cl,
		spectralExecutor:    spectralExecutor,
		executorId:          executorId,
	}
}

type docTaskProcessorImpl struct {
	docTaskRepo         repository.DocLintTaskRepository
	ruleSetRepository   repository.RuleSetRepository
	docResultRepository repository.DocResultRepository
	cl                  client.ApihubClient
	spectralExecutor    SpectralExecutor

	executorId string
}

// TODO: maybe need some fast track

// TODO: read from ticker chan or from events chan

func (d docTaskProcessorImpl) Start() {
	// TODO: multiple threads or not?

	utils.SafeAsync(func() {
		ticker := time.NewTicker(time.Second * 5)
		for range ticker.C {

			task, err := d.docTaskRepo.FindFreeDocTask(context.Background(), d.executorId)
			if err != nil {
				log.Errorf("Error finding free doc task: %s", err)
				continue
			}
			if task != nil {
				d.processDocTask(*task)
			}

			log.Infof("docTaskProcessorImpl running") // TODO: remove
		}
	})
}

func (d docTaskProcessorImpl) processDocTask(task entity.DocumentLintTask) {
	sc := secctx.CreateSystemContext()

	// TODO: get document metadata??

	data, err := d.cl.GetDocumentRawData(sc, task.PackageId, fmt.Sprintf("%s@%d", task.Version, task.Revision), task.FileSlug)
	if err != nil {
		// update task with error
		return
	}

	if len(data) == 0 {
		// update task with error
		return
	}

	tempDir := filepath.Join(os.TempDir(), task.Id)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		//d.updateTaskFailure(task, "failed to create temp dir: %v", err)
		// update task with error
		return
	}
	defer os.RemoveAll(tempDir)

	parts := strings.Split(task.FileId, "/")
	fileName := parts[len(parts)-1]

	filePath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		//d.updateTaskFailure(task, "failed to write document file: %v", err)
		// update task with error
		return
	}

	docHash := utils.CreateSHA256Hash(data)

	rs, err := d.ruleSetRepository.GetRulesetById(task.RulesetId)
	if err != nil {
		// update task with error
		return
	}
	rulesetPath := filepath.Join(tempDir, "ruleset.yaml") // TODO: extension from DB???!!!
	if err := os.WriteFile(rulesetPath, rs.Data, 0600); err != nil {
		//d.updateTaskFailure(task, "failed to write document file: %v", err)
		// update task with error
		return
	}

	if task.Linter == view.SpectralLinter {
		// TODO: check if the file is already linted!
		/*lintResultExists, err := d.docResultRepository.LintResultExists(docHash)
		if err != nil {
			// update task with error
			return
		}
		if lintResultExists {
			// TODO: do not need to run linter!
			// TODO: just save metadata about the run

			// TODO: what if run lint for the same revision twice????????????????
		}
		*/
		// it might take a long time due to linter lock or just long execution
		resultPath, calcTime, err := d.spectralExecutor.LintLocalDoc(filePath, rulesetPath)
		if err != nil {
			// update task with error
			return
		}

		result, resErr := os.ReadFile(resultPath)
		if resErr != nil {
			//log.Errorf("failed to read document validation file: %v", resErr.Error())
			//return nil, fmt.Sprintf("failed to read Spectral output file: %s", resErr.Error()), calculationTime.Milliseconds()

			// update task with error
			return
		}
		var report []interface{}
		err = json.Unmarshal(result, &report)
		if err != nil {
			//return nil, fmt.Sprintf("failed to unmarshal Spectral report: %v", err.Error()), calculationTime.Milliseconds()

			// update task with error
			return
		}

		log.Infof("%+v", calcTime)

		summary := calculateSpectralSummary(report)

		LinterVersion := d.spectralExecutor.GetLinterVersion()
		sumJson, err := json.Marshal(summary)
		if err != nil {
			// TODO
			return
		}

		var sumAsMap map[string]interface{}

		err = json.Unmarshal(sumJson, &sumAsMap)
		if err != nil {
			// TODO
			return
		}

		docEnt := entity.LintedDocument{
			PackageId:         task.PackageId,
			Version:           task.Version,
			Revision:          task.Revision,
			FileId:            task.FileId,
			FileName:          "todo", // FIXME!
			SpecificationType: "todo", // FIXME!
			Title:             "todo", // FIXME!
			RulesetId:         task.RulesetId,
			DataHash:          docHash,
			LintStatus:        view.StatusSuccess, // TODO calculate based on linter
			LintDetails:       "",
		}

		lintFileResult := entity.LintFileResult{
			DataHash:      docHash,
			RulesetId:     task.RulesetId,
			LinterVersion: LinterVersion,
			Data:          result,
			Summary:       sumAsMap,
		}

		err = d.docResultRepository.SaveLintResult(context.Background(), task.Id, docEnt, &lintFileResult)
		if err != nil {
			// TODO
			return
		}
		log.Println(lintFileResult)

		// TODO: trigger version task update? or just wait

	} else {
		// update task with error
		return
	}
}
