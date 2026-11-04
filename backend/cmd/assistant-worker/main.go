package main

import (
	"context"
	"os/signal"
	"syscall"
)

func main() {
	app, err := createApp()
	if err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err = app.Assistant.Work(ctx); err != nil {
		panic(err)
	}
}
