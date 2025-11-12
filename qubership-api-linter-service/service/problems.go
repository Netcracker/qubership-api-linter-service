package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
)

type ProblemsService interface {
	GenTaskRestDocProblems(ctx context.Context, packageId string, version string, revision int, slug string, docData string) ([]view.AIApiDocCatProblem, error)
	GetDocProblems(ctx context.Context, packageId string, version string, slug string) ([]view.AIApiDocCatProblem, error)
}

func NewProblemsService(apihubClient client.ApihubClient, llmClient client.LLMClient, localFileStore bool) ProblemsService {

	storage := make(map[string][]view.AIApiDocCatProblem)

	if localFileStore {
		data, err := os.ReadFile("problems_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &storage); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}
	}

	return &problemsServiceImpl{
		apihubClient:   apihubClient,
		llmClient:      llmClient,
		localFileStore: localFileStore,
		storage:        storage,
	}
}

type problemsServiceImpl struct {
	apihubClient client.ApihubClient
	llmClient    client.LLMClient

	localFileStore bool
	storage        map[string][]view.AIApiDocCatProblem
}

func (p problemsServiceImpl) GenTaskRestDocProblems(ctx context.Context, packageId string, version string, revision int, slug string, docData string) ([]view.AIApiDocCatProblem, error) {
	start := time.Now()
	problResp, err := p.llmClient.GenerateProblems(ctx, docData)
	if err != nil {
		return nil, err
	}

	catProbl, err := p.llmClient.CategorizeProblems(ctx, problResp)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(catProbl, func(a, b view.AIApiDocCatProblem) int {
		switch a.Severity {
		case view.PSError:
			switch b.Severity {
			case view.PSError:
				return 0
			case view.PSWarning:
				return -1
			case view.PSInfo:
				return -1
			}
		case view.PSWarning:
			switch b.Severity {
			case view.PSError:
				return 1
			case view.PSWarning:
				return 0
			case view.PSInfo:
				return -1
			}
		case view.PSInfo:
			switch b.Severity {
			case view.PSError:
				return 1
			case view.PSWarning:
				return 1
			case view.PSInfo:
				return 0
			}
		}
		return 0
	})

	key := packageId + "|" + fmt.Sprintf("%s@%d", version, revision) + "|" + slug
	p.storage[key] = catProbl

	log.Infof("time: %dms", time.Since(start).Milliseconds())
	log.Infof("problems: %+v", problResp)
	p.saveStorage()

	return catProbl, nil
}

func (p problemsServiceImpl) GetDocProblems(ctx context.Context, packageId string, version string, slug string) ([]view.AIApiDocCatProblem, error) {
	ver, rev, err := getVersionAndRevision(ctx, p.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	result, exists := p.storage[key]
	if !exists {
		return []view.AIApiDocCatProblem{}, nil
	}

	return result, nil
}

func (p problemsServiceImpl) saveStorage() {
	if !p.localFileStore {
		return
	}

	data, err := json.Marshal(p.storage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("problems_storage.json", data, 0644)
	if err != nil {
		log.Errorf("Failed to save storage to file: %+v", err)
	}
}
