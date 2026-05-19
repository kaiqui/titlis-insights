package repo

import (
	"context"
	"errors"

	"github.com/titlis/insights/internal/model"
)

var ErrNotFound = errors.New("not found")

type TemplateRepo interface {
	Get(ctx context.Context, tenantID int64, env model.Environment, crit model.Criticality) (model.HpaTemplate, error)
	List(ctx context.Context, tenantID int64) ([]model.HpaTemplate, error)
	Upsert(ctx context.Context, tpl model.HpaTemplate) (model.HpaTemplate, error)
	Delete(ctx context.Context, tenantID int64, env model.Environment, crit model.Criticality) error
}

type RecommendationLogRepo interface {
	Append(ctx context.Context, entry RecommendationLogEntry) error
}

type RecommendationLogEntry struct {
	TenantID    int64
	WorkloadUID string
	Environment model.Environment
	Source      model.RecommendationSource
	Confidence  float64
	DurationMS  int
	Cached      bool
}
