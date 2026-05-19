package repo

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/titlis/insights/internal/model"
)

type MemoryTemplateRepo struct {
	mu      sync.RWMutex
	nextID  int64
	entries map[string]model.HpaTemplate
}

func NewMemoryTemplateRepo() *MemoryTemplateRepo {
	return &MemoryTemplateRepo{entries: map[string]model.HpaTemplate{}}
}

func keyFor(tenantID int64, env model.Environment, crit model.Criticality) string {
	return fmtKey(tenantID, string(env), string(crit))
}

func fmtKey(tenantID int64, env, crit string) string {
	return env + "|" + crit + "|" + itoa(tenantID)
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func (r *MemoryTemplateRepo) Get(_ context.Context, tenantID int64, env model.Environment, crit model.Criticality) (model.HpaTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tpl, ok := r.entries[keyFor(tenantID, env, crit)]; ok {
		return tpl, nil
	}
	return model.HpaTemplate{}, ErrNotFound
}

func (r *MemoryTemplateRepo) List(_ context.Context, tenantID int64) ([]model.HpaTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.HpaTemplate, 0)
	for _, tpl := range r.entries {
		if tpl.TenantID == tenantID {
			out = append(out, tpl)
		}
	}
	return out, nil
}

func (r *MemoryTemplateRepo) Upsert(_ context.Context, tpl model.HpaTemplate) (model.HpaTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	k := keyFor(tpl.TenantID, tpl.Environment, tpl.Criticality)
	existing, exists := r.entries[k]
	if exists {
		tpl.ID = existing.ID
		tpl.CreatedAt = existing.CreatedAt
	} else {
		tpl.ID = atomic.AddInt64(&r.nextID, 1)
		tpl.CreatedAt = now
	}
	tpl.UpdatedAt = now
	r.entries[k] = tpl
	return tpl, nil
}

func (r *MemoryTemplateRepo) Delete(_ context.Context, tenantID int64, env model.Environment, crit model.Criticality) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyFor(tenantID, env, crit)
	if _, ok := r.entries[k]; !ok {
		return ErrNotFound
	}
	delete(r.entries, k)
	return nil
}

type NoopRecommendationLog struct{}

func (NoopRecommendationLog) Append(_ context.Context, _ RecommendationLogEntry) error { return nil }
