package recommend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/repo"
	"github.com/titlis/insights/internal/source"
)

const (
	defaultHeadroomPct  = 30.0
	defaultTargetCPUPct = 70
	minReplicasFloor    = 1
)

type Options struct {
	WindowDays    int
	MinConfidence float64
	HeadroomPct   float64
	TargetCPUPct  int
}

func DefaultOptions() Options {
	return Options{
		WindowDays:    30,
		MinConfidence: 0.7,
		HeadroomPct:   defaultHeadroomPct,
		TargetCPUPct:  defaultTargetCPUPct,
	}
}

type Recommender struct {
	src       source.MetricsSource
	templates repo.TemplateRepo
	opts      Options
	now       func() time.Time
}

func NewRecommender(src source.MetricsSource, templates repo.TemplateRepo, opts Options) *Recommender {
	if opts.WindowDays <= 0 {
		opts.WindowDays = 30
	}
	if opts.TargetCPUPct <= 0 {
		opts.TargetCPUPct = defaultTargetCPUPct
	}
	if opts.HeadroomPct <= 0 {
		opts.HeadroomPct = defaultHeadroomPct
	}
	return &Recommender{src: src, templates: templates, opts: opts, now: time.Now}
}

func (r *Recommender) WithClock(now func() time.Time) *Recommender {
	r.now = now
	return r
}

func (r *Recommender) ForWorkload(ctx context.Context, w model.WorkloadContext) (model.HpaRecommendation, error) {
	if w.HasDatadog {
		reco, err := r.fromUsage(ctx, w)
		if err == nil {
			return reco, nil
		}
		if !errors.Is(err, source.ErrSourceUnavailable) {
			return model.HpaRecommendation{}, err
		}
	}
	return r.fromTemplate(ctx, w)
}

func (r *Recommender) fromUsage(ctx context.Context, w model.WorkloadContext) (model.HpaRecommendation, error) {
	snap, err := r.src.UsageSnapshot(ctx, w, r.opts.WindowDays)
	if err != nil {
		return model.HpaRecommendation{}, err
	}
	if snap.ConfidenceFraction < r.opts.MinConfidence {
		return model.HpaRecommendation{}, source.ErrSourceUnavailable
	}
	minRep := int(math.Max(float64(snap.ReplicasPeak)*0.5, float64(minReplicasFloor)))
	maxRep := int(math.Ceil(float64(snap.ReplicasPeak) * (1 + r.opts.HeadroomPct/100)))
	if maxRep <= minRep {
		maxRep = minRep + 1
	}
	target := r.opts.TargetCPUPct
	if snap.CPUP95Pct > 0 && snap.CPUP95Pct < float64(target) {
		target = int(math.Min(math.Round(snap.CPUP95Pct+10), 90))
	}
	return model.HpaRecommendation{
		WorkloadUID:  w.WorkloadUID,
		MinReplicas:  minRep,
		MaxReplicas:  maxRep,
		TargetCPUPct: target,
		Source:       model.SourceDatadogP95,
		Confidence:   snap.ConfidenceFraction,
		WindowDays:   r.opts.WindowDays,
		ComputedAt:   r.now().UTC(),
		Notes: fmt.Sprintf("p95=%.1f%% peak=%d avg=%.1f samples=%d",
			snap.CPUP95Pct, snap.ReplicasPeak, snap.ReplicasAverage, snap.SamplesCount),
	}, nil
}

func (r *Recommender) fromTemplate(ctx context.Context, w model.WorkloadContext) (model.HpaRecommendation, error) {
	tpl, err := r.templates.Get(ctx, w.TenantID, w.Environment, w.Criticality)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return model.HpaRecommendation{
				WorkloadUID: w.WorkloadUID,
				Source:      model.SourceSkipped,
				ComputedAt:  r.now().UTC(),
				Notes:       fmt.Sprintf("no template for (%s, %s)", w.Environment, w.Criticality),
			}, nil
		}
		return model.HpaRecommendation{}, err
	}
	return model.HpaRecommendation{
		WorkloadUID:  w.WorkloadUID,
		MinReplicas:  tpl.MinReplicas,
		MaxReplicas:  tpl.MaxReplicas,
		TargetCPUPct: tpl.TargetCPUPct,
		TargetMemPct: tpl.TargetMemPct,
		Source:       model.SourceTemplate,
		Confidence:   1.0,
		ComputedAt:   r.now().UTC(),
		Notes:        fmt.Sprintf("template (%s, %s)", w.Environment, w.Criticality),
	}, nil
}
