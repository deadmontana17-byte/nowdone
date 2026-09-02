// Command worker runs the cron jobs: nightly streak recalculation (hourly
// check, per backend rules) and per-minute reminder dispatch.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	// Embed the IANA timezone database so reminder times can be rendered in each
	// user's local zone even on a minimal container image.
	_ "time/tzdata"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"

	"nowdone/internal/config"
	"nowdone/internal/db"
	"nowdone/internal/logger"
	"nowdone/internal/repository"
	"nowdone/internal/service"
	"nowdone/internal/worker"
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

	userRepo := repository.NewUserRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)

	streakSvc := service.NewStreakService(userRepo, taskRepo, log)
	notifySvc := service.NewNotificationService(api)
	reminderWorker := worker.NewReminderWorker(taskRepo, userRepo, notifySvc, log)

	c := cron.New(cron.WithLocation(service.MoscowLocation))

	// Streak recalculation: run every hour, per backend rule. RecalculateAll is
	// a full recompute of each user's consecutive all-tasks-done days, so
	// running it several times a day always converges on the same value; the
	// hourly cadence just keeps the number fresh as the day's tasks get ticked
	// off and as midnight passes in each user's timezone.
	if _, err := c.AddFunc("@hourly", func() {
		if err := streakSvc.RecalculateAll(context.Background()); err != nil {
			log.Error("streak recalculation failed", "error", err)
		}
	}); err != nil {
		log.Error("schedule streak job", "error", err)
		os.Exit(1)
	}

	// Run once at startup so a freshly (re)deployed worker doesn't leave streaks
	// stale until the top of the next hour.
	go func() {
		if err := streakSvc.RecalculateAll(context.Background()); err != nil {
			log.Error("initial streak recalculation failed", "error", err)
		}
	}()

	// Reminder dispatch: check every minute for due, unsent reminders.
	if _, err := c.AddFunc("* * * * *", func() {
		reminderWorker.Run(context.Background())
	}); err != nil {
		log.Error("schedule reminder job", "error", err)
		os.Exit(1)
	}

	c.Start()
	log.Info("worker started: streak (hourly) + reminders (every minute)")

	<-ctx.Done()
	log.Info("shutting down worker")
	stopCtx := c.Stop()
	<-stopCtx.Done()
}
