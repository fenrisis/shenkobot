package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"

	"shenkobot/internal/handler"
	applogger "shenkobot/internal/logger"
	"shenkobot/internal/migrations"
	"shenkobot/internal/repository"
	"shenkobot/internal/service"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	zlog, err := applogger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer func() { _ = zlog.Sync() }()

	zlog.Info("starting",
		zap.String("log_level", cfg.LogLevel),
		zap.String("log_format", cfg.LogFormat),
		zap.Int64("chat_id", cfg.ChatID),
		zap.Int("reminder_hours", cfg.ReminderHours),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gormLog := applogger.NewGorm(zlog, gormlogger.Warn)
	repo, err := repository.New(cfg.DatabaseURL(), gormLog)
	if err != nil {
		zlog.Fatal("db connect failed", zap.Error(err))
	}
	defer func() {
		if err := repo.Close(); err != nil {
			zlog.Error("db close failed", zap.Error(err))
		}
	}()

	if err := migrations.Run(repo.DB(), zlog); err != nil {
		zlog.Fatal("migrate failed", zap.Error(err))
	}

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		zlog.Fatal("bot init failed", zap.Error(err))
	}
	zlog.Info("telegram authorized", zap.String("bot_username", api.Self.UserName), zap.Int64("bot_id", api.Self.ID))

	sender := handler.NewTelegramSender(api, zlog)
	svc := service.New(repo, sender, cfg.ChatID, cfg.Question, cfg.ReminderInterval(), zlog)
	h := handler.New(api, svc, cfg.ChatID, zlog)

	go svc.RunScheduler(ctx)
	go h.Run(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	zlog.Info("shutting down", zap.String("signal", sig.String()))
}
