package recommend_test

import (
	"context"
	"testing"
	"time"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/recommend"
	"github.com/titlis/insights/internal/repo"
	"github.com/titlis/insights/internal/source/memory"
)

func fixedClock() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

func TestRecommender_FromUsage_HappyPath(t *testing.T) {
	src := memory.NewSource()
	src.Set("uid-1", model.UsageSnapshot{
		WindowDays:         30,
		CPUP95Pct:          60,
		ReplicasPeak:       8,
		ReplicasAverage:    4.5,
		SamplesCount:       8640,
		ConfidenceFraction: 0.95,
	})
	r := recommend.NewRecommender(src, repo.NewMemoryTemplateRepo(), recommend.DefaultOptions()).
		WithClock(fixedClock)
	reco, err := r.ForWorkload(context.Background(), model.WorkloadContext{
		TenantID:    1,
		WorkloadUID: "uid-1",
		Environment: model.EnvProd,
		Criticality: model.CriticalityHigh,
		HasDatadog:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reco.Source != model.SourceDatadogP95 {
		t.Fatalf("expected datadog source, got %s", reco.Source)
	}
	if reco.MinReplicas < 1 {
		t.Fatalf("min replicas must be >= 1, got %d", reco.MinReplicas)
	}
	if reco.MaxReplicas <= reco.MinReplicas {
		t.Fatalf("max must be > min, got min=%d max=%d", reco.MinReplicas, reco.MaxReplicas)
	}
	if reco.TargetCPUPct <= 0 || reco.TargetCPUPct > 90 {
		t.Fatalf("target_cpu out of range: %d", reco.TargetCPUPct)
	}
	if reco.Confidence < 0.9 {
		t.Fatalf("expected high confidence, got %f", reco.Confidence)
	}
}

func TestRecommender_LowConfidence_FallsBackToTemplate(t *testing.T) {
	src := memory.NewSource()
	src.Set("uid-low", model.UsageSnapshot{
		ConfidenceFraction: 0.3,
		CPUP95Pct:          40,
		ReplicasPeak:       2,
	})
	templates := repo.NewMemoryTemplateRepo()
	_, _ = templates.Upsert(context.Background(), model.HpaTemplate{
		TenantID: 1, Environment: model.EnvProd, Criticality: model.CriticalityHigh,
		MinReplicas: 2, MaxReplicas: 6, TargetCPUPct: 70,
	})
	r := recommend.NewRecommender(src, templates, recommend.DefaultOptions()).WithClock(fixedClock)
	reco, err := r.ForWorkload(context.Background(), model.WorkloadContext{
		TenantID: 1, WorkloadUID: "uid-low",
		Environment: model.EnvProd, Criticality: model.CriticalityHigh,
		HasDatadog: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reco.Source != model.SourceTemplate {
		t.Fatalf("expected template fallback, got %s", reco.Source)
	}
	if reco.MinReplicas != 2 || reco.MaxReplicas != 6 || reco.TargetCPUPct != 70 {
		t.Fatalf("unexpected reco from template: %+v", reco)
	}
}

func TestRecommender_NoDatadog_NoTemplate_ReturnsSkipped(t *testing.T) {
	r := recommend.NewRecommender(memory.NewSource(), repo.NewMemoryTemplateRepo(), recommend.DefaultOptions()).
		WithClock(fixedClock)
	reco, err := r.ForWorkload(context.Background(), model.WorkloadContext{
		TenantID: 1, WorkloadUID: "uid-dev",
		Environment: model.EnvDev, Criticality: model.CriticalityMedium,
		HasDatadog: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reco.Source != model.SourceSkipped {
		t.Fatalf("expected skipped, got %s", reco.Source)
	}
}

func TestRecommender_NoDatadog_WithTemplate_UsesTemplate(t *testing.T) {
	templates := repo.NewMemoryTemplateRepo()
	_, _ = templates.Upsert(context.Background(), model.HpaTemplate{
		TenantID: 7, Environment: model.EnvDev, Criticality: model.CriticalityMedium,
		MinReplicas: 1, MaxReplicas: 3, TargetCPUPct: 80,
	})
	r := recommend.NewRecommender(memory.NewSource(), templates, recommend.DefaultOptions()).WithClock(fixedClock)
	reco, err := r.ForWorkload(context.Background(), model.WorkloadContext{
		TenantID: 7, WorkloadUID: "uid-dev",
		Environment: model.EnvDev, Criticality: model.CriticalityMedium,
		HasDatadog: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reco.Source != model.SourceTemplate {
		t.Fatalf("expected template, got %s", reco.Source)
	}
	if reco.MinReplicas != 1 || reco.MaxReplicas != 3 || reco.TargetCPUPct != 80 {
		t.Fatalf("template mismatch: %+v", reco)
	}
}
