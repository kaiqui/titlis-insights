package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/repo"
)

func TestMemoryTemplateRepo_UpsertGet(t *testing.T) {
	r := repo.NewMemoryTemplateRepo()
	ctx := context.Background()

	tpl := model.HpaTemplate{TenantID: 1, Environment: model.EnvDev, Criticality: model.CriticalityMedium,
		MinReplicas: 1, MaxReplicas: 3, TargetCPUPct: 80}
	saved, err := r.Upsert(ctx, tpl)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("expected id assigned")
	}
	got, err := r.Get(ctx, 1, model.EnvDev, model.CriticalityMedium)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != saved.ID {
		t.Fatalf("expected same id, got %d", got.ID)
	}

	updated := tpl
	updated.MaxReplicas = 5
	again, err := r.Upsert(ctx, updated)
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if again.ID != saved.ID {
		t.Fatalf("expected same id on update; got %d vs %d", again.ID, saved.ID)
	}
	if again.MaxReplicas != 5 {
		t.Fatalf("expected updated value; got %d", again.MaxReplicas)
	}
}

func TestMemoryTemplateRepo_ListIsolation(t *testing.T) {
	r := repo.NewMemoryTemplateRepo()
	ctx := context.Background()
	_, _ = r.Upsert(ctx, model.HpaTemplate{TenantID: 1, Environment: model.EnvDev, Criticality: model.CriticalityMedium, MinReplicas: 1, MaxReplicas: 2, TargetCPUPct: 80})
	_, _ = r.Upsert(ctx, model.HpaTemplate{TenantID: 2, Environment: model.EnvDev, Criticality: model.CriticalityMedium, MinReplicas: 1, MaxReplicas: 5, TargetCPUPct: 70})

	got1, _ := r.List(ctx, 1)
	got2, _ := r.List(ctx, 2)
	if len(got1) != 1 || len(got2) != 1 {
		t.Fatalf("expected 1 each, got %d / %d", len(got1), len(got2))
	}
	if got1[0].MaxReplicas == got2[0].MaxReplicas {
		t.Fatal("expected different templates per tenant")
	}
}

func TestMemoryTemplateRepo_Delete(t *testing.T) {
	r := repo.NewMemoryTemplateRepo()
	ctx := context.Background()
	_, _ = r.Upsert(ctx, model.HpaTemplate{TenantID: 1, Environment: model.EnvDev, Criticality: model.CriticalityMedium, MinReplicas: 1, MaxReplicas: 2, TargetCPUPct: 80})
	if err := r.Delete(ctx, 1, model.EnvDev, model.CriticalityMedium); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := r.Delete(ctx, 1, model.EnvDev, model.CriticalityMedium); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
