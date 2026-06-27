package api

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// Validation patterns for shell-safe inputs
var (
	// Package names: letters, digits, dots, dashes, plus, colons (version specs)
	pkgNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+:~-]*$`)
	// Docker container/image names
	dockerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-/:@]*$`)
	// Helm chart names: repo/chart format
	helmChartRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/]*$`)
	// Generic identifier: alphanumeric + dash + dot
	identRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]*$`)
	// Version strings: v1.2.3+rke2r1 format
	versionRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+[+\-][a-zA-Z0-9.+\-]*$`)
	// URL: http(s)://...
	urlRe = regexp.MustCompile(`^https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+$`)
	// IP address
	ipRe = regexp.MustCompile(`^[0-9a-fA-F.:]+$`)
)

func validatePackageName(name string) bool {
	return pkgNameRe.MatchString(name)
}

func validateDockerName(name string) bool {
	return dockerNameRe.MatchString(name)
}

func validateHelmChart(name string) bool {
	return helmChartRe.MatchString(name)
}

func validateVersion(ver string) bool {
	if ver == "" {
		return true
	}
	return versionRe.MatchString(ver)
}

func validateURL(u string) bool {
	if u == "" {
		return true
	}
	return urlRe.MatchString(u)
}

// sanitizeForShell removes or rejects characters that allow shell injection.
// Returns the sanitized string, or empty string if injection detected.
func sanitizeForShell(s string) string {
	// Reject if contains any shell metacharacters
	for _, ch := range s {
		switch ch {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\n', '\r', '\\', '"', '\'', '!', '#':
			return ""
		}
	}
	return s
}

// requireValidation wraps a handler to validate before proceeding
func requireValidation(c *gin.Context, field string, valid bool) bool {
	if !valid {
		c.AbortWithStatusJSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: fmt.Sprintf("invalid %s: contains forbidden characters", field),
		})
		return false
	}
	return true
}
