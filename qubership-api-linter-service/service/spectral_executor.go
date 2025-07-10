package service

import (
	"bytes"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SpectralExecutor interface {
	LintLocalDoc(docPath string, rulesetPath string) (string, int64, error)
	GetLinterVersion() string
}

func NewSpectralExecutor(spectralBinPath string) (SpectralExecutor, error) {
	spectralVersion, err := detectSpectralVersion(spectralBinPath)
	if err != nil {
		return nil, err
	}
	return &spectralExecutorImpl{spectralBinPath: spectralBinPath, semaphore: utils.NewSemaphore(1), spectralVersion: spectralVersion}, nil
}

type spectralExecutorImpl struct {
	spectralBinPath string
	semaphore       *utils.Semaphore
	spectralVersion string
}

func (s *spectralExecutorImpl) LintLocalDoc(docPath string, rulesetPath string) (string, int64, error) {
	s.semaphore.Acquire()
	defer s.semaphore.Release()

	resultPath := docPath
	if filepath.Ext(resultPath) != "" {
		resultPath = strings.TrimSuffix(resultPath, "."+filepath.Ext(resultPath))
	}
	resultPath += "-result.json"

	var args []string
	args = append(args, "lint")
	args = append(args, docPath)
	args = append(args, "--ruleset")
	args = append(args, rulesetPath)
	args = append(args, "-q")
	args = append(args, "-f")
	args = append(args, "json")
	args = append(args, "-o.json")
	args = append(args, resultPath)

	cmd := exec.Command(s.spectralBinPath, args...)
	// TODO: ??????
	/*inBuffer := bytes.Buffer{}
	inBuffer.Write(validationData)
	cmd.Stdin = &inBuffer*/
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	calculationTime := time.Since(start) /*Error running Spectral!
	Use --verbose flag to print the error stack.
	Error #1: Unexpected token (Note that you need plugins to import files that are not JavaScript)
	*/
	if err != nil {
		//spectral process exits with status 1 if validation contains at least one error...
		if err.Error() != "exit status 1" {
			return "", calculationTime.Milliseconds(), fmt.Errorf("failed to get Spectral report: %v", err.Error())
		}
	}

	return resultPath, calculationTime.Milliseconds(), nil
}

func (s *spectralExecutorImpl) GetLinterVersion() string {
	return s.spectralVersion
}

func detectSpectralVersion(spectralBinPath string) (string, error) {
	if spectralBinPath == "" {
		return "", fmt.Errorf("spectral executor path is not set (SPECTRAL_BIN_PATH env)")
	}
	var spectralVersion string
	args := []string{"--version"}
	cmd := exec.Command(spectralBinPath, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting spectral version: %s", err)
	}
	spectralVersion = out.String()
	spectralVersion = strings.TrimSpace(spectralVersion)
	return spectralVersion, nil
}
