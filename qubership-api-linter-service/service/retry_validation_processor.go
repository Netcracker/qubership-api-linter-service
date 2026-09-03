package service

import (
	"context"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/config"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	log "github.com/sirupsen/logrus"
)

const retryTaskCreatedBy = "system_retry"

type RetryValidationProcessor interface {
	Start()
}

func NewRetryValidationProcessor(verRepo repository.VersionLintTaskRepository, cfg config.ValidationRetryConfig, versionTaskNotify chan<- struct{}) RetryValidationProcessor {
	return &retryValidationProcessorImpl{
		verRepo:           verRepo,
		cfg:               cfg,
		versionTaskNotify: versionTaskNotify,
	}
}

type retryValidationProcessorImpl struct {
	verRepo           repository.VersionLintTaskRepository
	cfg               config.ValidationRetryConfig
	versionTaskNotify chan<- struct{}
}

func (p *retryValidationProcessorImpl) Start() {
	if !p.cfg.Enabled {
		log.Info("retryValidationProcessor: automatic retry of failed validations is disabled")
		return
	}

	utils.SafeAsync(func() {
		p.runSweepLoop()
	})
	log.Info("retryValidationProcessor started")
}

func (p *retryValidationProcessorImpl) runSweepLoop() {
	t := time.NewTicker(p.cfg.Interval)
	defer t.Stop()

	for range t.C {
		p.retryFailedValidation()
	}
}

func (p *retryValidationProcessorImpl) retryFailedValidation() {
	ctx := context.Background()
	start := time.Now()

	retryTasks, err := p.verRepo.ScheduleRetriesForFailedValidations(ctx, repository.ValidationRetryParams{
		MaxAttempts: p.cfg.MaxAttempts,
		MaxAge:      p.cfg.MaxAge,
		RetryDelay:  p.cfg.RetryDelay,
		BatchSize:   p.cfg.BatchSize,
		CreatedBy:   retryTaskCreatedBy,
	})

	if err != nil {
		log.Errorf("retryValidationProcessor: failed to schedule retries for failed validations: %s", err)
		return
	}
	if len(retryTasks) == 0 {
		log.Debugf("retryValidationProcessor: no failed validations to retry")
		return
	}

	for _, task := range retryTasks {
		log.Infof("retryValidationProcessor: scheduled retry %d of %d for [ %s | %s@%d ], task id = %s",
			task.RetryCount, p.cfg.MaxAttempts, task.PackageId, task.Version, task.Revision, task.Id)
	}

	select {
	case p.versionTaskNotify <- struct{}{}:
	default:
	}

	log.Infof("retryValidationProcessor: scheduled %d validation retry task(s) in %dms", len(retryTasks), time.Since(start).Milliseconds())
}
