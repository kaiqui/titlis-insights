package recommend

import (
	"math"

	"github.com/titlis/insights/internal/model"
)

func WithinTolerance(current model.HpaCurrent, reco model.HpaRecommendation, tolerancePct float64) bool {
	if reco.Source == model.SourceSkipped {
		return true
	}
	if !withinPct(float64(current.MinReplicas), float64(reco.MinReplicas), tolerancePct) {
		return false
	}
	if !withinPct(float64(current.MaxReplicas), float64(reco.MaxReplicas), tolerancePct) {
		return false
	}
	if !withinPct(float64(current.TargetCPUPct), float64(reco.TargetCPUPct), tolerancePct) {
		return false
	}
	return true
}

func withinPct(actual, expected, tolerancePct float64) bool {
	if expected == 0 {
		return actual == 0
	}
	delta := math.Abs(actual-expected) / math.Abs(expected) * 100
	return delta <= tolerancePct
}

func DeltaPct(current model.HpaCurrent, reco model.HpaRecommendation) float64 {
	if reco.Source == model.SourceSkipped {
		return 0
	}
	d := 0.0
	d = math.Max(d, absPct(float64(current.MinReplicas), float64(reco.MinReplicas)))
	d = math.Max(d, absPct(float64(current.MaxReplicas), float64(reco.MaxReplicas)))
	d = math.Max(d, absPct(float64(current.TargetCPUPct), float64(reco.TargetCPUPct)))
	return d
}

func absPct(actual, expected float64) float64 {
	if expected == 0 {
		if actual == 0 {
			return 0
		}
		return 100
	}
	return math.Abs(actual-expected) / math.Abs(expected) * 100
}
