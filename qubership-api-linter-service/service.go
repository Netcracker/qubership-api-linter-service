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

package main

import (
	"context"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/security"
	"github.com/google/uuid"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/controller"
	"github.com/Netcracker/qubership-api-linter-service/service"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func main() {
	systemInfoService, err := service.NewSystemInfoService()
	if err != nil {
		panic(err)
	}

	basePath := systemInfoService.GetBasePath()
	r := mux.NewRouter().SkipClean(true).UseEncodedPath()

	creds := systemInfoService.GetCredsFromEnv()
	cp := db.NewConnectionProvider(creds)
	initSrv := makeServer(systemInfoService, r)

	readyChan := make(chan bool)
	migrationPassedChan := make(chan bool)
	initSrvStoppedChan := make(chan bool)

	dbMigrationService, err := service.NewDBMigrationService(cp, systemInfoService)
	if err != nil {
		log.Error("Failed create dbMigrationService: " + err.Error())
		panic("Failed create dbMigrationService: " + err.Error())
	}

	go func(initSrvStoppedChan chan bool) { // Do not use safe async here to enable panic
		log.Debugf("Starting init srv")
		_ = initSrv.ListenAndServe()
		log.Debugf("Init srv closed")
		initSrvStoppedChan <- true
		close(initSrvStoppedChan)
	}(initSrvStoppedChan)

	go func(migrationReadyChan chan bool) { // Do not use safe async here to enable panic
		passed := <-migrationPassedChan
		err := initSrv.Shutdown(context.Background())
		if err != nil {
			log.Fatalf("Failed to shutdown initial server")
		}
		if !passed {
			log.Fatalf("Stopping server since migration failed")
		}
		migrationReadyChan <- true
		close(migrationReadyChan)
		close(migrationPassedChan)
	}(readyChan)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() { // Do not use safe async here to enable panic
		defer wg.Done()

		_, _, _, err := dbMigrationService.Migrate(basePath)
		if err != nil {
			log.Error("Failed perform DB migration: " + err.Error())
			time.Sleep(time.Second * 10) // Give a chance to read the unrecoverable error
			panic("Failed perform DB migration: " + err.Error())
		}

		migrationPassedChan <- true
	}()

	wg.Wait()
	_ = <-initSrvStoppedChan // wait for the init srv to stop to avoid multiple servers started race condition
	log.Infof("Migration step passed, continue initialization")
	////

	olricProvider, err := client.NewOlricProvider()
	if err != nil {
		log.Error("Failed to create olricProvider: " + err.Error())
		panic("Failed to create olricProvider: " + err.Error())
	}

	apihubClient := client.NewApihubClient(systemInfoService.GetAPIHubUrl(), systemInfoService.GetApihubAccessToken())

	err = security.SetupGoGuardian(apihubClient)
	if err != nil {
		log.Fatalf("Failed to setup go guardian: %s", err.Error())
	}
	log.Info("go_guardian is set up")

	executorId := uuid.NewString()

	versionLintTaskRepository := repository.NewVersionLintTaskRepository(cp)
	docLintTaskRepository := repository.NewDocLintTaskRepository(cp)
	ruleSetRepository := repository.NewRuleSetRepository(cp)
	docResultRepository := repository.NewDocResultRepository(cp)

	linterSelectorService := service.NewLinterSelectorService(ruleSetRepository)

	versionTaskProcessor := service.NewVersionTaskProcessor(versionLintTaskRepository, docLintTaskRepository, apihubClient, linterSelectorService, executorId)
	spectralExecutor, err := service.NewSpectralExecutor(systemInfoService.GetSpectralBinPath())
	if err != nil {
		log.Fatalf("Failed to create Spectral executor: %s", err.Error())
	}

	docTaskProcessor := service.NewDocTaskProcessor(docLintTaskRepository, ruleSetRepository, docResultRepository, apihubClient, spectralExecutor, executorId)

	validationService := service.NewValidationService(versionLintTaskRepository, versionTaskProcessor, executorId)
	publishEventListener := service.NewPublishEventListener(olricProvider, validationService)

	validationController := controller.NewValidationController(validationService)
	rulesetController := controller.NewRulesetController()
	healthController := controller.NewHealthController(readyChan)

	r.HandleFunc("/api/validate", security.Secure(validationController.ValidateAPI)).Methods(http.MethodPost)
	r.HandleFunc("/ruleset", rulesetController.GetRulesetFile).Methods(http.MethodGet)
	r.HandleFunc("/rulesetjs", rulesetController.GetJSRulesetFile).Methods(http.MethodGet)
	r.HandleFunc("/rulesetjson", rulesetController.GetJsonRulesetFile).Methods(http.MethodGet)

	r.HandleFunc("/live", healthController.HandleLiveRequest).Methods(http.MethodGet)
	r.HandleFunc("/ready", healthController.HandleReadyRequest).Methods(http.MethodGet)

	publishEventListener.Start()
	docTaskProcessor.Start()

	knownPathPrefixes := []string{
		"/api/",
		"/ruleset/",     //TODO: remove
		"/rulesetjs/",   //TODO: remove
		"/rulesetjson/", //TODO: remove
		"/live/",
		"/ready/",
	}
	for _, prefix := range knownPathPrefixes {
		//add routing for unknown paths with known path prefixes
		r.PathPrefix(prefix).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Warnf("Requested unknown endpoint: %v %v", r.Method, r.RequestURI)
			controller.RespondWithCustomError(w, &exception.CustomError{
				Status:  http.StatusMisdirectedRequest,
				Message: "Requested unknown endpoint",
			})
		})
	}

	debug.SetGCPercent(30)

	srv := makeServer(systemInfoService, r)

	log.Fatalf("%v", srv.ListenAndServe())
}

func makeServer(systemInfoService service.SystemInfoService, r *mux.Router) *http.Server {
	listenAddr := systemInfoService.GetListenAddress()

	log.Infof("Listen addr = %s", listenAddr)

	var corsOptions []handlers.CORSOption

	corsOptions = append(corsOptions, handlers.AllowedHeaders([]string{"Connection", "Accept-Encoding", "Content-Encoding", "X-Requested-With", "Content-Type", "Authorization"}))

	allowedOrigin := systemInfoService.GetOriginAllowed()
	if allowedOrigin != "" {
		corsOptions = append(corsOptions, handlers.AllowedOrigins([]string{allowedOrigin}))
	}
	corsOptions = append(corsOptions, handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"}))

	return &http.Server{
		Handler:      handlers.CompressHandler(handlers.CORS(corsOptions...)(r)),
		Addr:         listenAddr,
		WriteTimeout: 600 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
}
