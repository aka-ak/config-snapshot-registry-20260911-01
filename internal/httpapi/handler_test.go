package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/service"
	"example.com/config-snapshot-registry/internal/store"
)

func TestCreateAndGetSnapshot(t *testing.T) {
	handler := New(service.New(store.NewMemory()))
	body, _ := json.Marshal(domain.Snapshot{ID: "snap-1", Service: "search", Environment: "prod", CapturedAt: time.Now().UTC(), Values: map[string]any{"replicas": 2}})
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/v1/snapshots", bytes.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/snapshots/snap-1", nil))
	if get.Code != http.StatusOK || !bytes.Contains(get.Body.Bytes(), []byte("snap-1")) {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
}

func TestDiffRequiresBothIDs(t *testing.T) {
	handler := New(service.New(store.NewMemory()))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/diffs?from=a", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestImpactAssessmentEndpoint(t *testing.T) {
	handler := New(service.New(store.NewMemory()))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, snapshot := range []domain.Snapshot{
		{ID: "snap-1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{"timeout": 3}, Dependencies: []string{"auth"}},
		{ID: "snap-2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{"timeout": 5}, Dependencies: []string{"auth"}},
		{ID: "snap-3", Service: "auth", Environment: "prod", CapturedAt: base, Values: map[string]any{}},
		{ID: "snap-4", Service: "checkout", Environment: "prod", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"payments"}},
	} {
		putSnapshot(t, handler, snapshot)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/impact?service=payments&environment=prod&from=snap-1&to=snap-2", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"path":"timeout"`)) {
		t.Fatalf("expected timeout change in body=%s", recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"service":"checkout"`)) {
		t.Fatalf("expected checkout as affected in body=%s", recorder.Body.String())
	}
}

func TestImpactRequiresAllParameters(t *testing.T) {
	handler := New(service.New(store.NewMemory()))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/impact?service=payments&from=snap-1&to=snap-2", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestImpactReportsSnapshotErrors(t *testing.T) {
	handler := New(service.New(store.NewMemory()))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	putSnapshot(t, handler, domain.Snapshot{ID: "snap-1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{"timeout": 3}})
	putSnapshot(t, handler, domain.Snapshot{ID: "snap-2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{"timeout": 5}, Dependencies: []string{"auth"}})
	putSnapshot(t, handler, domain.Snapshot{ID: "snap-3", Service: "payments", Environment: "staging", CapturedAt: base, Values: map[string]any{}})

	cases := []struct {
		name   string
		target string
		status int
	}{
		{"missing snapshot", "/v1/impact?service=payments&environment=prod&from=snap-1&to=unknown", http.StatusNotFound},
		{"scope mismatch", "/v1/impact?service=payments&environment=prod&from=snap-1&to=snap-3", http.StatusBadRequest},
		{"incomplete dependencies", "/v1/impact?service=payments&environment=prod&from=snap-1&to=snap-2", http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status=%d want %d body=%s", recorder.Code, tc.status, recorder.Body.String())
			}
		})
	}
}

func putSnapshot(t *testing.T, handler http.Handler, snapshot domain.Snapshot) {
	t.Helper()
	body, _ := json.Marshal(snapshot)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/snapshots", bytes.NewReader(body)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("put %s: status=%d body=%s", snapshot.ID, recorder.Code, recorder.Body.String())
	}
}
