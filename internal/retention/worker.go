package retention

import (
	"context"
	"time"

	"example.com/config-snapshot-registry/internal/service"
)

type Worker struct {
	Service  *service.Service
	Interval time.Duration
	MaxAge   time.Duration
}

func (w Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_ = w.Service.PruneBefore(ctx, now.Add(-w.MaxAge))
		}
	}
}
