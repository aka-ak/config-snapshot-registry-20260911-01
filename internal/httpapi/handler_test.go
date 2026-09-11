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
