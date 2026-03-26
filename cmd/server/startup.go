package main

import (
	"context"

	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func StartBatchLoop(lc fx.Lifecycle, s scheduler.Scheduler, log *zap.SugaredLogger) {
	runCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Info("Scheduler batch loop started")
				s.Batch(runCtx)
				log.Info("Scheduler batch loop stopped")
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return nil
		},
	})
}
