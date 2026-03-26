package main

import (
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/handler"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	connect "github.com/qujing226/mini-llm-serve/internal/transport"
	"github.com/qujing226/mini-llm-serve/internal/worker"
	"github.com/spf13/pflag"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func main() {
	parseFlags := fx.Annotate(
		func() (confPath string) {
			pflag.StringVarP(&confPath, "conf", "c", "server.toml", "Path to the configuration file (e.g., --config=./server.toml/server.yml/server.yaml/server.json)")
			pflag.Parse()
			return confPath
		},
		fx.ResultTags(`name:"confPath"`),
	)

	app := fx.New(
		fx.Provide(
			parseFlags,
			fx.Annotate(conf.NewConfFromPath, fx.ParamTags(`name:"confPath"`)),
			newLogger),
		fx.WithLogger(func(log *zap.SugaredLogger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log.Desugar()}
		}),
		fx.Options(),
		fx.Provide(
			worker.NewExecutors,
			worker.NewWorker,
			scheduler.NewQueue,
			scheduler.NewScheduler,
			handler.NewInferenceHandle,
			connect.NewLLMServingServer,
		),
		fx.Invoke(
			connect.StartInferenceServer,
			StartBatchLoop,
		),
	)
	app.Run()
}
