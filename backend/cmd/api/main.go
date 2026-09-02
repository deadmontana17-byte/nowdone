// Command api runs the NowDone HTTP API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embed the IANA timezone database so user.Location() works regardless of
	// whether the host/container ships /usr/share/zoneinfo.
	_ "time/tzdata"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/internal/config"
	"nowdone/internal/db"
	"nowdone/internal/handlers"
	"nowdone/internal/logger"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.RunMigrations(cfg.DBDSN, cfg.MigrationsPath); err != nil {
		log.Error("run migrations", "error", err)
		os.Exit(1)
	}

	pool, err := db.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("connect db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	authCodeRepo := repository.NewAuthCodeRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)
	taskTypeRepo := repository.NewTaskTypeRepository(pool)
	noteRepo := repository.NewNoteRepository(pool)

	s3Svc, err := service.NewS3Service(ctx, cfg)
	if err != nil {
		log.Error("init s3 service", "error", err)
		os.Exit(1)
	}

	// Lightweight Bot API client so the API can push PIN-reset codes straight to
	// a user's Telegram chat (no bot /start, no deep link).
	botAPI, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Error("init telegram bot api", "error", err)
		os.Exit(1)
	}

	authSvc := service.NewAuthService(userRepo, authCodeRepo, cfg, service.NewTelegramCodeSender(botAPI))
	taskSvc := service.NewTaskService(taskRepo, s3Svc, log)
	taskTypeSvc := service.NewTaskTypeService(taskTypeRepo)
	noteSvc := service.NewNoteService(noteRepo, s3Svc, log)
	if err := s3Svc.HealthCheck(ctx); err != nil {
		// Non-fatal: the API still serves everything except attachment uploads.
		log.Warn("s3 health check failed, attachment uploads may not work", "error", err)
	} else {
		log.Info("s3 storage reachable", "bucket", cfg.S3BucketName, "endpoint", cfg.S3Endpoint)
	}

	authHandler := handlers.NewAuthHandler(authSvc, userRepo, cfg, log)

	router := handlers.NewRouter(handlers.Deps{
		Cfg:       cfg,
		Log:       log,
		Auth:      authSvc,
		Users:     userRepo,
		Tasks:     taskSvc,
		TaskTypes: taskTypeSvc,
		Notes:     noteSvc,
		S3:        s3Svc,
		AuthH:     authHandler,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api server starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down api server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
}
