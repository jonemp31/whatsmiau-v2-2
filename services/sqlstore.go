package services

import (
	"context"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.uber.org/zap"
)

var sqlStoreInstance *sqlstore.Container

func SQLStore() *sqlstore.Container {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()

	if sqlStoreInstance == nil {
		container, err := sqlstore.New(ctx, env.Env.DBDialect, env.Env.DBURL, nil)
		if err != nil {
			zap.L().Error("failed to start sqlstore", zap.Error(err), zap.String("db_dialect", env.Env.DBDialect), zap.String("db_url", env.Env.DBURL))
			panic(fmt.Sprintf("failed to start sqlstore: %v", err)) // Mantém panic aqui pois é crítico para a aplicação
		}

		zap.L().Info("successfully connected to database", zap.String("db_dialect", env.Env.DBDialect))
		sqlStoreInstance = container
	}

	return sqlStoreInstance
}
