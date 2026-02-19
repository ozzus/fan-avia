package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

func parsePositiveIntQuery(r *http.Request, key string) (value string, present bool, errMsg string) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return "", false, ""
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return "", true, "invalid"
	}

	return strconv.FormatInt(parsed, 10), true, ""
}
