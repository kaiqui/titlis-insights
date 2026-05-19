package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	thttp "github.com/titlis/insights/internal/http"
	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/observability"
	"github.com/titlis/insights/internal/recommend"
	"github.com/titlis/insights/internal/repo"
	"github.com/titlis/insights/internal/source/memory"
)

func setupServer(t *testing.T) (http.Handler, *memory.Source, repo.TemplateRepo) {
	t.Helper()
	src := memory.NewSource()
	tpls := repo.NewMemoryTemplateRepo()
	rec := recommend.NewRecommender(src, tpls, recommend.DefaultOptions())
	h := thttp.NewHandlers(rec, tpls, repo.NoopRecommendationLog{})
	log := observability.NewLogger("error", "json")
	return thttp.NewRouter(h, "sek", log), src, tpls
}

func TestHandler_Health_NoAuth(t *testing.T) {
	srv, _, _ := setupServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandler_Recommendation_AuthRequired(t *testing.T) {
	srv, _, _ := setupServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/recommendations/hpa?tenant_id=1&workload_uid=x", nil)
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestHandler_Recommendation_FromTemplate(t *testing.T) {
	srv, _, tpls := setupServer(t)
	_, _ = tpls.Upsert(context.Background(), model.HpaTemplate{
		TenantID: 1, Environment: model.EnvDev, Criticality: model.CriticalityMedium,
		MinReplicas: 1, MaxReplicas: 4, TargetCPUPct: 80,
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/recommendations/hpa?tenant_id=1&workload_uid=foo&environment=dev&criticality=medium", nil)
	req.Header.Set("X-Internal-Secret", "sek")
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var reco model.HpaRecommendation
	if err := json.NewDecoder(rr.Body).Decode(&reco); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if reco.Source != model.SourceTemplate || reco.MinReplicas != 1 {
		t.Fatalf("unexpected: %+v", reco)
	}
}

func TestHandler_Recommendation_Skipped_NoData(t *testing.T) {
	srv, _, _ := setupServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/recommendations/hpa?tenant_id=1&workload_uid=x&environment=dev&criticality=medium", nil)
	req.Header.Set("X-Internal-Secret", "sek")
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var reco model.HpaRecommendation
	_ = json.NewDecoder(rr.Body).Decode(&reco)
	if reco.Source != model.SourceSkipped {
		t.Fatalf("expected skipped, got %s", reco.Source)
	}
}

func TestHandler_Templates_Upsert_Validation(t *testing.T) {
	srv, _, _ := setupServer(t)
	body := strings.NewReader(`{"environment":"dev","criticality":"medium","min_replicas":0,"max_replicas":3,"target_cpu_pct":70}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenants/1/hpa-templates", body)
	req.Header.Set("X-Internal-Secret", "sek")
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid replicas, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Templates_Upsert_OK(t *testing.T) {
	srv, _, _ := setupServer(t)
	body := strings.NewReader(`{"environment":"dev","criticality":"medium","min_replicas":1,"max_replicas":3,"target_cpu_pct":80}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/tenants/1/hpa-templates", body)
	req.Header.Set("X-Internal-Secret", "sek")
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}
