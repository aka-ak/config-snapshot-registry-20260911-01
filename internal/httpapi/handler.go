package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/service"
)

type Handler struct {
	service *service.Service
}

func New(svc *service.Service) http.Handler {
	h := &Handler{service: svc}
	return http.HandlerFunc(h.handle)
}

func (h *Handler) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/snapshots":
		h.createSnapshot(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/snapshots":
		h.listSnapshots(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/snapshots/"):
		h.getSnapshot(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/diffs":
		h.diff(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/impact":
		h.impact(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/audit":
		h.audit(w, r)
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

func (h *Handler) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var input domain.Snapshot
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.PutSnapshot(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (h *Handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListSnapshots(r.Context(), service.ListOptions{
		Service: r.URL.Query().Get("service"), Environment: r.URL.Query().Get("environment"),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/snapshots/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}
	record, err := h.service.GetSnapshot(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) diff(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("from") == "" || query.Get("to") == "" {
		writeError(w, http.StatusBadRequest, "from and to are required")
		return
	}
	result, err := h.service.Diff(r.Context(), query.Get("from"), query.Get("to"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) impact(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("service") == "" || query.Get("environment") == "" || query.Get("from") == "" || query.Get("to") == "" {
		writeError(w, http.StatusBadRequest, "service, environment, from and to are required")
		return
	}
	result, err := h.service.AssessImpact(r.Context(), query.Get("service"), query.Get("environment"), query.Get("from"), query.Get("to"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.Audit(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidSnapshot):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrSnapshotConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrSnapshotNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrScopeMismatch):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrIncompleteDependencies):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
