package main

import (
	"context"
	"database/sql"
	"github.com/caarlos0/env/v11"
	"github.com/getsentry/sentry-go"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"goboxer/internal"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type applicationConfig struct {
	AppName     string `env:"APP_NAME"`
	Environment string `env:"ENVIRONMENT"`
	SentryDsn   string `env:"SENTRY_DSN"`
	Release     string `env:"RELEASE"`
	LogstashDsn string `env:"LOGSTASH_DSN"`
}

var sugaredLogger *zap.SugaredLogger
var appConfig *applicationConfig

func init() {
	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)
	sugaredLogger = logger.Sugar()
	// logger

	appConfig = &applicationConfig{}
	opts := env.Options{RequiredIfNoDef: true}
	if err := env.ParseWithOptions(appConfig, opts); err != nil {
		sugaredLogger.Error("Configuration can not be parsed", zap.Error(err))
		os.Exit(1)
	}
	sugaredLogger = sugaredLogger.With("appName", appConfig.AppName)
	// app configuration

	sentryErr := sentry.Init(sentry.ClientOptions{
		Dsn:              appConfig.SentryDsn,
		AttachStacktrace: true,
		Release:          appConfig.Release,
		Environment:      appConfig.Environment,
		SampleRate:       1,
	})
	if sentryErr != nil {
		sugaredLogger.Error("Sentry init error", zap.Error(sentryErr))
	}
	defer sentry.Flush(2 * time.Second)
	// sentry
}

func main() {
	sugaredLogger.Info("Application started")
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-c
		switch sig {
		case os.Interrupt:
			sugaredLogger.Info("Stopped application", zap.Any("signal", sig))
			cancel()
			signal.Stop(c)
			os.Exit(1)
		case syscall.SIGQUIT:
			sugaredLogger.Info("Stopped application", zap.Any("signal", sig))
			cancel()
			signal.Stop(c)
			os.Exit(1)
		default:
			sugaredLogger.Info("Stopped application", zap.Any("signal", sig))
			cancel()
			signal.Stop(c)
			os.Exit(1)
		}
	}()

	func() {
		var wg sync.WaitGroup

		srvProvider := &internal.StubServiceProvider{}

		for {
			registeredServices := srvProvider.GetServicesList()
			for serviceName, serviceConfig := range registeredServices {
				sugaredLogger.Info("Registering service", zap.String("serviceName", serviceName))
				db, err := sql.Open(serviceConfig.Driver, serviceConfig.Dsn)
				if err != nil {
					sugaredLogger.Error("Failed to open database", zap.Error(err))
					continue
				}

				err = db.Ping()
				if err != nil {
					sugaredLogger.Error("Failed to ping database", zap.Error(err))
					continue
				}

				_, err = db.Query("SELECT 1 FROM `message_outbox`")
				if err != nil {
					sugaredLogger.Error("Cannot probe `message_outbox` table", zap.Error(err))
					continue
				}

				wg.Add(1)
				go run(context.WithValue(ctx, "serviceName", serviceName), db, &wg)
			}

			wg.Wait()
			time.Sleep(1 * time.Second)
		}
	}()
}

func run(ctx context.Context, db *sql.DB, wg *sync.WaitGroup) {
	processLogger := sugaredLogger.With("service", ctx.Value("serviceName").(string))

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		processLogger.Error("Failed to begin transaction", zap.Error(err))
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
		_ = db.Close()
		wg.Done()
	}(tx)

	var messages []internal.OutboxMessage

	rows, err := db.Query(
		"SELECT id, body, meta, status, attempt_count FROM message_outbox WHERE status = ? LIMIT 10 FOR UPDATE",
		"new",
	)

	if err != nil {
		processLogger.Error("Failed to query database", zap.Error(err))
	}

	defer rows.Close()

	for rows.Next() {
		var message internal.OutboxMessage
		err = rows.Scan(&message.ID, &message.Body, &message.Meta, &message.Status, &message.AttemptCount)
		if err != nil {
			processLogger.Error("Failed to scan message", zap.Error(err))
		}
		messages = append(messages, message)
	}
	processLogger.Info("Got messages", zap.Int("count", len(messages)))

	err = tx.Commit()
	if err != nil {
		processLogger.Error("Failed to commit transaction", zap.Error(err))
	}

	processLogger.Info("Commited. Iteration done")
}
