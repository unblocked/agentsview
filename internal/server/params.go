package server

import (
	"fmt"
	"net/http"
	"strconv"
)

// parseIntParam reads an integer query parameter from r.
// Returns (value, true) on success, or writes a 400 error and
// returns (0, false) if the parameter is present but not a valid
// integer.  When the parameter is absent, returns (0, true).
func parseIntParam(
	w http.ResponseWriter, r *http.Request, name string,
) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid %s parameter", name))
		return 0, false
	}
	return v, true
}

// clampLimit applies a default and upper bound to a limit value.
func clampLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
