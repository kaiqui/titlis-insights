package memory

import (
	"context"
	"sync"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/source"
)

type Source struct {
	mu        sync.RWMutex
	snapshots map[string]model.UsageSnapshot
	failures  map[string]bool
}

func NewSource() *Source {
	return &Source{
		snapshots: map[string]model.UsageSnapshot{},
		failures:  map[string]bool{},
	}
}

func (s *Source) Name() string { return "memory" }

func (s *Source) Set(workloadUID string, snap model.UsageSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[workloadUID] = snap
}

func (s *Source) SetFailure(workloadUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[workloadUID] = true
}

func (s *Source) UsageSnapshot(_ context.Context, w model.WorkloadContext, _ int) (model.UsageSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.failures[w.WorkloadUID] {
		return model.UsageSnapshot{}, source.ErrSourceUnavailable
	}
	if snap, ok := s.snapshots[w.WorkloadUID]; ok {
		return snap, nil
	}
	return model.UsageSnapshot{}, source.ErrSourceUnavailable
}
