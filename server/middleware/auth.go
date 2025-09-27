package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.uber.org/zap"
)

func Auth(ctx echo.Context, next echo.HandlerFunc) error {
	gotApikey := ctx.Request().Header.Get("apikey")

	// Em produção, sempre exigir API_KEY configurada
	if len(env.Env.ApiKey) == 0 {
		zap.L().Warn("API_KEY not configured - allowing all requests (INSECURE)")
		return next(ctx)
	}

	if gotApikey == "" {
		zap.L().Warn("request without API key", zap.String("ip", ctx.RealIP()))
		return echo.NewHTTPError(http.StatusUnauthorized, "API key required")
	}

	if gotApikey != env.Env.ApiKey {
		zap.L().Warn("invalid API key", zap.String("ip", ctx.RealIP()))
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid API key")
	}

	return next(ctx)
}

type simplifiedMiddleware func(c echo.Context, next echo.HandlerFunc) error

func Simplify(handler simplifiedMiddleware) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			return handler(ctx, next)
		}
	}
}
