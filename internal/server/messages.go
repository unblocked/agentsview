package server

import (
	"math"
	"net/http"

	dbpkg "github.com/wesm/agentsview/internal/db"
)

func (s *Server) handleGetMessages(
	w http.ResponseWriter, r *http.Request,
) {
	sessionID := r.PathValue("id")

	limit, ok := parseIntParam(w, r, "limit")
	if !ok {
		return
	}
	limit = clampLimit(limit, dbpkg.DefaultMessageLimit, dbpkg.MaxMessageLimit)

	asc := r.URL.Query().Get("direction") != "desc"

	from := 0
	if r.URL.Query().Get("from") != "" {
		var ok bool
		from, ok = parseIntParam(w, r, "from")
		if !ok {
			return
		}
	} else if !asc {
		from = math.MaxInt32
	}

	msgs, err := s.db.GetMessages(
		r.Context(), sessionID, from, limit, asc,
	)
	if err != nil {
		if handleContextError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": msgs,
		"count":    len(msgs),
	})
}

func (s *Server) handleGetMinimap(
	w http.ResponseWriter, r *http.Request,
) {
	sessionID := r.PathValue("id")
	from := 0
	if r.URL.Query().Get("from") != "" {
		var ok bool
		from, ok = parseIntParam(w, r, "from")
		if !ok {
			return
		}
	}
	max := 0
	if r.URL.Query().Get("max") != "" {
		var ok bool
		max, ok = parseIntParam(w, r, "max")
		if !ok {
			return
		}
		if max <= 0 {
			writeError(w, http.StatusBadRequest,
				"invalid max parameter")
			return
		}
	}

	entries, err := s.db.GetMinimapFrom(
		r.Context(), sessionID, from,
	)
	if err != nil {
		if handleContextError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if max > 0 {
		entries = dbpkg.SampleMinimap(entries, max)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}
