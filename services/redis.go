package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.uber.org/zap"
)

var redisInstance *redis.Client

func Redis() *redis.Client {
	if redisInstance == nil {
		// Tentar conectar com retry
		var instance *redis.Client
		var err error

		for i := 0; i < 5; i++ {
			instance, err = NewRedis()
			if err == nil {
				redisInstance = instance
				break
			}

			zap.L().Warn("failed to connect to redis, retrying...",
				zap.Error(err),
				zap.Int("attempt", i+1),
				zap.Int("max_attempts", 5))

			if i < 4 { // Não esperar na última tentativa
				time.Sleep(time.Second * 2)
			}
		}

		if err != nil {
			zap.L().Fatal("failed to connect to redis after 5 attempts", zap.Error(err))
		}
	}

	return redisInstance
}

func NewRedis() (*redis.Client, error) {
	opt := &redis.Options{
		Addr:     env.Env.RedisURL,
		Password: env.Env.RedisPassword,
		DB:       0,
	}

	if env.Env.RedisTLS {
		opt.TLSConfig = &tls.Config{}
	}

	client := redis.NewClient(opt)
	if err := client.Ping(context.Background()).Err(); err != nil {
		zap.L().Error("failed to connect to redis", zap.Error(err), zap.String("redis_url", env.Env.RedisURL))
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	zap.L().Info("successfully connected to redis", zap.String("redis_url", env.Env.RedisURL))
	return client, nil
}
