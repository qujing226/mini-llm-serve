package main

import (
	"github.com/qujing226/mini-llm-serve/internal/conf"
	connect "github.com/qujing226/mini-llm-serve/internal/transport"
	"github.com/spf13/pflag"
	"go.uber.org/fx"
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
		fx.Options(),
		fx.Provide(
			connect.NewLLMServingServer,
		),
		fx.Invoke(
			connect.StartInferenceServer,
		),
	)
	app.Run()
}
