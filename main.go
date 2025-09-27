package main

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/lib/converter"
	"github.com/verbeux-ai/whatsmiau/lib/logger"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/routes"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
)

func main() {
	if err := env.Load(); err != nil {
		panic(err)
	}

	if err := logger.StartLogger(); err != nil {
		log.Fatalln(err)
	}

	converterSvc := converter.NewConverterService(env.Env.ConverterWorkers, time.Second*time.Duration(env.Env.ConverterHTTPTimeout))

	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	whatsmiau.LoadMiau(ctx, services.SQLStore(), converterSvc)

	app := echo.New()
	app.Pre(middleware.Recover())
	app.Pre(middleware.RemoveTrailingSlash())
	app.Pre(middleware.CORS())

	routes.Load(app)

	port := ":" + env.Env.Port
	zap.L().Info("starting server...", zap.String("port", port))

	s := &http2.Server{}
	if err := app.StartH2CServer(port, s); err != nil {
		zap.L().Fatal("failed to start server", zap.Error(err))
	}
}
