package main

import (
	"backend/internal/config"
	"backend/internal/handler"
	"backend/internal/repository/local"
	"backend/internal/repository/postgres"
	"backend/internal/service"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "backend/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

//	@title			Subscriptions API
//	@version		1.0
//	@description	REST-сервис для агрегации данных об онлайн-подписках пользователей.
//	@description	Даты подписок передаются с точностью до месяца в формате MM-YYYY, стоимость — целое число рублей.
//	@host			localhost:8080
//	@BasePath		/
//	@schemes		http

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.GetLogLevel()}))
	slog.SetDefault(logger)

	repo, cleanup, err := newRepository(cfg)
	if err != nil {
		log.Fatalf("init repository: %v", err)
	}
	defer cleanup()

	subscriptionsService := service.NewSubscriptionsService(repo, logger)
	subscriptionsHandler := handler.NewSubscriptionsHandler(subscriptionsService, logger)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := repo.Ping(pingCtx); err != nil {
		log.Fatalf("storage unreachable: %v", err)
	}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	subscriptionsHandler.Register(app)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.ServerPort, "storage", cfg.Storage)
		serverErr <- app.Listen(fmt.Sprintf(":%d", cfg.ServerPort))
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("server: %v", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}

	logger.Info("server stopped")
}

func newRepository(cfg *config.Config) (service.SubscriptionsRepository, func(), error) {
	switch cfg.Storage {
	case config.StoragePostgres:
		pool, err := pgxpool.New(context.Background(), cfg.GetDBDSN())
		if err != nil {
			return nil, nil, fmt.Errorf("pgx pool: %w", err)
		}
		return postgres.NewSubscriptionsRepository(pool), pool.Close, nil
	case config.StorageLocal:
		return local.NewSubscriptionsRepository(), func() {}, nil
	default:
		return nil, nil, fmt.Errorf("unknown storage type: %q", cfg.Storage)
	}
}
