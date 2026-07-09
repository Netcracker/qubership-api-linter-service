package utils

import (
	"net"
	"net/http"
	"strings"
)

const (
	xForwardedForHeader = "X-Forwarded-For"
	xRealIPHeader       = "X-Real-IP"
	unknownRequestorIP  = "unknown"
)

func RequestorIP(r *http.Request) string {
	if r == nil {
		return unknownRequestorIP
	}

	if ip := firstValidIP(r.Header.Get(xForwardedForHeader)); ip != "" {
		return ip
	}
	if ip := validIP(r.Header.Get(xRealIPHeader)); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if ip := validIP(host); ip != "" {
			return ip
		}
	}
	if ip := validIP(r.RemoteAddr); ip != "" {
		return ip
	}
	return unknownRequestorIP
}

func firstValidIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		if ip := validIP(part); ip != "" {
			return ip
		}
	}
	return ""
}

func validIP(value string) string {
	ip := strings.TrimSpace(value)
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}
