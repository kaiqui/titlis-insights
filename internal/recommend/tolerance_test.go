package recommend_test

import (
	"testing"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/recommend"
)

func TestWithinTolerance(t *testing.T) {
	reco := model.HpaRecommendation{
		MinReplicas: 3, MaxReplicas: 10, TargetCPUPct: 70,
		Source: model.SourceDatadogP95,
	}
	tests := []struct {
		name string
		cur  model.HpaCurrent
		want bool
	}{
		{"exact", model.HpaCurrent{MinReplicas: 3, MaxReplicas: 10, TargetCPUPct: 70}, true},
		{"within-10pct", model.HpaCurrent{MinReplicas: 3, MaxReplicas: 11, TargetCPUPct: 75}, true},
		{"outside-30pct-max", model.HpaCurrent{MinReplicas: 3, MaxReplicas: 14, TargetCPUPct: 70}, false},
		{"outside-min-zero", model.HpaCurrent{MinReplicas: 1, MaxReplicas: 10, TargetCPUPct: 70}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := recommend.WithinTolerance(tc.cur, reco, 20.0)
			if got != tc.want {
				t.Fatalf("expected %v got %v for %+v", tc.want, got, tc.cur)
			}
		})
	}
}

func TestWithinTolerance_SkippedAlwaysTrue(t *testing.T) {
	reco := model.HpaRecommendation{Source: model.SourceSkipped}
	if !recommend.WithinTolerance(model.HpaCurrent{}, reco, 20.0) {
		t.Fatal("skipped recommendation should always be in tolerance")
	}
}

func TestDeltaPct(t *testing.T) {
	reco := model.HpaRecommendation{
		MinReplicas: 2, MaxReplicas: 10, TargetCPUPct: 70,
		Source: model.SourceDatadogP95,
	}
	cur := model.HpaCurrent{MinReplicas: 2, MaxReplicas: 10, TargetCPUPct: 84}
	d := recommend.DeltaPct(cur, reco)
	if d < 19 || d > 21 {
		t.Fatalf("expected delta ~20%%, got %.2f", d)
	}
}
