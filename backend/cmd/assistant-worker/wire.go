//go:build wireinject

package main

import (
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/google/wire"
)

func createApp() (*App, error) {
	wire.Build(config.ProviderSet, log.ProviderSet, usecase.ProviderSet, wire.Struct(new(App), "*"))
	return &App{}, nil
}

type App struct{ Assistant *usecase.AssistantUsecase }
