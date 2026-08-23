package service

import (
	"context"
	"log"
	"time"
)

type RefreshWorker struct {
	Service  *RefreshService
	Interval time.Duration
	Logger   *log.Logger
}

func NewRefreshWorker(service *RefreshService, interval time.Duration, logger *log.Logger) *RefreshWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	return &RefreshWorker{Service: service, Interval: interval, Logger: logger}
}

func (w *RefreshWorker) Run(ctx context.Context) {
	if w == nil || w.Service == nil || w.Service.Repository == nil {
		return
	}
	w.runOnce(ctx)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *RefreshWorker) runOnce(ctx context.Context) {
	searches, err := w.Service.Repository.FindSearchesNeedingRefresh(ctx)
	if err != nil {
		w.logf("worker failed to find searches error=%v", err)
		return
	}
	for _, search := range searches {
		if err := w.Service.RefreshSearch(ctx, search.ID); err != nil {
			w.logf("worker refresh failed search_id=%d error=%v", search.ID, err)
		}
	}
}

func (w *RefreshWorker) logf(format string, args ...any) {
	if w.Logger != nil {
		w.Logger.Printf(format, args...)
	}
}
