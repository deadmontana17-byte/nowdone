// Command bot runs the NowDone Telegram bot (deep-link auth + AI task chat).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/internal/config"
	"nowdone/internal/db"
	"nowdone/internal/logger"
	"nowdone/internal/repository"
	"nowdone/internal/service"
	"nowdone/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}

	log := logger.New(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("connect db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Error("init telegram bot api", "error", err)
		os.Exit(1)
	}
	api.Debug = cfg.Env == "development"

	userRepo := repository.NewUserRepository(pool)
	authCodeRepo := repository.NewAuthCodeRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)
	taskTypeRepo := repository.NewTaskTypeRepository(pool)
	noteRepo := repository.NewNoteRepository(pool)

	// The bot now uploads photo/video/file attachments sent in chat to the same
	// object storage the API uses, and cleans them up when a task/note is
	// deleted — so it needs a real S3 client. A health-check failure is
	// non-fatal: everything except attachment handling still works.
	s3Svc, err := service.NewS3Service(ctx, cfg)
	if err != nil {
		log.Error("init s3 service", "error", err)
		os.Exit(1)
	}
	if err := s3Svc.HealthCheck(ctx); err != nil {
		log.Warn("s3 health check failed, bot attachment uploads may not work", "error", err)
	}

	// nil ResetCodeSender: PIN resets are only ever originated from the website,
	// so the bot never needs to push a reset code itself.
	authSvc := service.NewAuthService(userRepo, authCodeRepo, cfg, nil)
	taskSvc := service.NewTaskService(taskRepo, s3Svc, log)
	taskTypeSvc := service.NewTaskTypeService(taskTypeRepo)
	noteSvc := service.NewNoteService(noteRepo, s3Svc, log)
	openaiSvc := service.NewOpenAIService(cfg.OpenAIAPIKey)

	bot := telegram.New(api, authSvc, taskSvc, taskTypeSvc, noteSvc, userRepo, openaiSvc, s3Svc, log)

	log.Info("starting telegram bot")
	if err := bot.Run(ctx); err != nil && err != context.Canceled {
		log.Error("bot stopped with error", "error", err)
		os.Exit(1)
	}
}
