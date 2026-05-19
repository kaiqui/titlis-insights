package source

import (
	"context"
	"errors"

	"github.com/titlis/insights/internal/model"
)

var ErrSourceUnavailable = errors.New("metrics source unavailable")

type MetricsSource interface {
	Name() string
	UsageSnapshot(ctx context.Context, w model.WorkloadContext, windowDays int) (model.UsageSnapshot, error)
}
