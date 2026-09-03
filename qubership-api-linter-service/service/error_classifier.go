package service

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/view"
)

var retriableValidationErrorMarkers = []string{
	"connection reset by peer",
	"connection refused",
	"broken pipe",
	"unexpected eof",
	"eof",
	"i/o timeout",
	"timeout awaiting response headers",
	"tls handshake timeout",
	"context deadline exceeded",
	"no such host",
	"server misbehaving",
	"too many requests",
	"rate limit",
	"service unavailable",
	"bad gateway",
	"gateway timeout",
	"internal server error",
	"driver: bad connection",
	"the database system is",
}

func ClassifyError(err error) view.ErrorKind {
	if err == nil {
		return ""
	}

	if isRetriableError(err) {
		return view.ErrorKindRetriable
	}

	return view.ErrorKindNotRetriableError
}

func isRetriableError(err error) bool {
	if customErr, ok := errors.AsType[*exception.CustomError](err); ok && isRetriableHttpStatus(customErr.Status) {
		return true
	}

	return matchesRetriableErrorMarker(err.Error())
}

func isRetriableHttpStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func matchesRetriableErrorMarker(message string) bool {
	lowered := strings.ToLower(message)
	for _, marker := range retriableValidationErrorMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}
