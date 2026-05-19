package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/titlis/insights/internal/model"
)

// PGTemplateRepo persists HPA environment templates in titlis_insights.hpa_environment_templates.
type PGTemplateRepo struct {
	db *pgxpool.Pool
}

func NewPGTemplateRepo(db *pgxpool.Pool) *PGTemplateRepo { return &PGTemplateRepo{db: db} }

func (r *PGTemplateRepo) Get(ctx context.Context, tenantID int64, env model.Environment, crit model.Criticality) (model.HpaTemplate, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, environment, criticality,
		       min_replicas, max_replicas, target_cpu_pct, COALESCE(target_mem_pct, 0),
		       updated_at
		FROM titlis_insights.hpa_environment_templates
		WHERE tenant_id = $1 AND environment = $2 AND criticality = $3`,
		tenantID, string(env), string(crit),
	)
	return scanTemplate(row)
}

func (r *PGTemplateRepo) List(ctx context.Context, tenantID int64) ([]model.HpaTemplate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, environment, criticality,
		       min_replicas, max_replicas, target_cpu_pct, COALESCE(target_mem_pct, 0),
		       updated_at
		FROM titlis_insights.hpa_environment_templates
		WHERE tenant_id = $1
		ORDER BY environment, criticality`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.HpaTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PGTemplateRepo) Upsert(ctx context.Context, tpl model.HpaTemplate) (model.HpaTemplate, error) {
	now := time.Now().UTC()
	var memPct *int
	if tpl.TargetMemPct > 0 {
		memPct = &tpl.TargetMemPct
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO titlis_insights.hpa_environment_templates
			(tenant_id, environment, criticality, min_replicas, max_replicas,
			 target_cpu_pct, target_mem_pct, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, environment, criticality) DO UPDATE SET
			min_replicas   = EXCLUDED.min_replicas,
			max_replicas   = EXCLUDED.max_replicas,
			target_cpu_pct = EXCLUDED.target_cpu_pct,
			target_mem_pct = EXCLUDED.target_mem_pct,
			updated_at     = EXCLUDED.updated_at
		RETURNING id, tenant_id, environment, criticality,
		          min_replicas, max_replicas, target_cpu_pct, COALESCE(target_mem_pct, 0),
		          updated_at`,
		tpl.TenantID, string(tpl.Environment), string(tpl.Criticality),
		tpl.MinReplicas, tpl.MaxReplicas, tpl.TargetCPUPct, memPct, now,
	)
	return scanTemplate(row)
}

func (r *PGTemplateRepo) Delete(ctx context.Context, tenantID int64, env model.Environment, crit model.Criticality) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM titlis_insights.hpa_environment_templates
		WHERE tenant_id = $1 AND environment = $2 AND criticality = $3`,
		tenantID, string(env), string(crit),
	)
	return err
}

func scanTemplate(row pgx.Row) (model.HpaTemplate, error) {
	var t model.HpaTemplate
	var env, crit string
	err := row.Scan(&t.ID, &t.TenantID, &env, &crit,
		&t.MinReplicas, &t.MaxReplicas, &t.TargetCPUPct, &t.TargetMemPct,
		&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HpaTemplate{}, ErrNotFound
	}
	if err != nil {
		return model.HpaTemplate{}, err
	}
	t.Environment = model.Environment(env)
	t.Criticality = model.Criticality(crit)
	return t, nil
}

// PGRecommendationLog persists recommendation audit entries in titlis_insights.recommendation_log.
type PGRecommendationLog struct {
	db *pgxpool.Pool
}

func NewPGRecommendationLog(db *pgxpool.Pool) *PGRecommendationLog {
	return &PGRecommendationLog{db: db}
}

func (r *PGRecommendationLog) Append(ctx context.Context, entry RecommendationLogEntry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO titlis_insights.recommendation_log
			(tenant_id, workload_uid, environment, source, confidence, duration_ms, cached)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.TenantID, entry.WorkloadUID, string(entry.Environment),
		string(entry.Source), entry.Confidence, entry.DurationMS, entry.Cached,
	)
	return err
}
