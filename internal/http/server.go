package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/titlis/insights/internal/observability"
)

func NewRouter(h *Handlers, internalSecret string, log *observability.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestLogger(log))
	r.Get("/health", h.Health)
	r.Group(func(r chi.Router) {
		r.Use(InternalSecretAuth(internalSecret))
		r.Get("/v1/recommendations/hpa", h.GetHpaRecommendation)
		r.Get("/v1/tenants/{tenantID}/hpa-templates", h.ListTemplates)
		r.Put("/v1/tenants/{tenantID}/hpa-templates", h.UpsertTemplate)
		r.Get("/v1/datadog/probe", h.DatadogProbe)
	})
	return r
}
