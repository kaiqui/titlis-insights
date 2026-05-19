package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/repo"
)

type Recommender interface {
	ForWorkload(ctx context.Context, w model.WorkloadContext) (model.HpaRecommendation, error)
}

// DatadogProber validates that a tenant's Datadog credentials are functional.
// Returns (ok, reason, error): ok=true means credentials are valid.
type DatadogProber interface {
	Probe(ctx context.Context, tenantID int64) (ok bool, reason string, err error)
}

type Handlers struct {
	Templates repo.TemplateRepo
	Recommend Recommender
	LogRepo   repo.RecommendationLogRepo
	Now       func() time.Time
	Prober    DatadogProber // optional; nil → returns "not_configured"
}

func NewHandlers(rec Recommender, t repo.TemplateRepo, l repo.RecommendationLogRepo) *Handlers {
	if l == nil {
		l = repo.NoopRecommendationLog{}
	}
	return &Handlers{Templates: t, Recommend: rec, LogRepo: l, Now: time.Now}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseEnvCrit(env, crit string) (model.Environment, model.Criticality, error) {
	switch model.Environment(env) {
	case model.EnvDev, model.EnvStaging, model.EnvProd:
	default:
		return "", "", errors.New("invalid environment: must be dev, hml or prd")
	}
	switch model.Criticality(crit) {
	case model.CriticalityLow, model.CriticalityMedium, model.CriticalityHigh, model.CriticalityCritical:
	default:
		return "", "", errors.New("invalid criticality: must be low, medium, high or critical")
	}
	return model.Environment(env), model.Criticality(crit), nil
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "titlis-insights"})
}

func (h *Handlers) GetHpaRecommendation(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID, err := parseInt64(q.Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "tenant_id required")
		return
	}
	wUID := q.Get("workload_uid")
	if wUID == "" {
		writeError(w, http.StatusBadRequest, "workload_uid required")
		return
	}
	env := q.Get("environment")
	if env == "" {
		env = "prd"
	}
	crit := q.Get("criticality")
	if crit == "" {
		crit = "medium"
	}
	envT, critT, err := parseEnvCrit(env, crit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hasDD := q.Get("has_datadog") == "true"
	wctx := model.WorkloadContext{
		TenantID:       tenantID,
		WorkloadUID:    wUID,
		DeploymentName: q.Get("deployment_name"),
		Namespace:      q.Get("namespace"),
		Cluster:        q.Get("cluster"),
		Environment:    envT,
		Criticality:    critT,
		HasDatadog:     hasDD,
	}
	start := time.Now()
	reco, err := h.Recommend.ForWorkload(r.Context(), wctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.LogRepo.Append(r.Context(), repo.RecommendationLogEntry{
		TenantID:    tenantID,
		WorkloadUID: wUID,
		Environment: envT,
		Source:      reco.Source,
		Confidence:  reco.Confidence,
		DurationMS:  int(time.Since(start).Milliseconds()),
	})
	writeJSON(w, http.StatusOK, reco)
}

type templateRequest struct {
	Environment  string `json:"environment"`
	Criticality  string `json:"criticality"`
	MinReplicas  int    `json:"min_replicas"`
	MaxReplicas  int    `json:"max_replicas"`
	TargetCPUPct int    `json:"target_cpu_pct"`
	TargetMemPct int    `json:"target_mem_pct,omitempty"`
}

func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseInt64(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	out, err := h.Templates.List(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out == nil {
		out = []model.HpaTemplate{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) UpsertTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseInt64(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var req templateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.MinReplicas < 1 || req.MaxReplicas < req.MinReplicas {
		writeError(w, http.StatusBadRequest, "invalid replicas range")
		return
	}
	if req.TargetCPUPct < 30 || req.TargetCPUPct > 95 {
		writeError(w, http.StatusBadRequest, "target_cpu_pct must be in [30,95]")
		return
	}
	envT, critT, err := parseEnvCrit(req.Environment, req.Criticality)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tpl, err := h.Templates.Upsert(r.Context(), model.HpaTemplate{
		TenantID:     tenantID,
		Environment:  envT,
		Criticality:  critT,
		MinReplicas:  req.MinReplicas,
		MaxReplicas:  req.MaxReplicas,
		TargetCPUPct: req.TargetCPUPct,
		TargetMemPct: req.TargetMemPct,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *Handlers) DatadogProbe(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseInt64(r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "tenant_id required")
		return
	}
	if h.Prober == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant_id": tenantID,
			"status":    "not_configured",
			"reason":    "datadog source not enabled in this deployment",
		})
		return
	}
	ok, reason, err := h.Prober.Probe(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := "error"
	if ok {
		status = "ok"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID,
		"status":    status,
		"reason":    reason,
	})
}
