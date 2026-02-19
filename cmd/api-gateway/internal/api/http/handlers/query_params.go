package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

func parsePositiveIntQuery(r *http.Request, key string) (value string, present bool, errMsg string) {
	values, exists := r.URL.Query()[key]
	if !exists {
		return "", false, ""
	}
	if len(values) == 0 {
		return "", true, "invalid"
	}

	raw := strings.TrimSpace(values[0])
	if raw == "" {
		return "", true, "invalid"
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return "", true, "invalid"
	}

	return strconv.FormatInt(parsed, 10), true, ""
}
