package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"shenkobot/internal/repository"
)

// Sender abstracts the Telegram-side send call so the service stays
// independent of the specific bot library.
type Sender interface {
	Send(chatID int64, text string) (messageID int64, err error)
}

type Service struct {
	repo     *repository.Repository
	sender   Sender
	chatID   int64
	question string
	interval time.Duration
	log      *zap.Logger
}

func New(repo *repository.Repository, sender Sender, chatID int64, question string, interval time.Duration, log *zap.Logger) *Service {
	return &Service{
		repo:     repo,
		sender:   sender,
		chatID:   chatID,
		question: question,
		interval: interval,
		log:      log.With(zap.String("component", "service"), zap.Int64("chat_id", chatID)),
	}
}

func (s *Service) AskAll(ctx context.Context) {
	op := s.log.With(zap.String("op", "ask_all"))
	targets, err := s.repo.Targets(ctx)
	if err != nil {
		op.Error("load targets failed", zap.Error(err))
		return
	}
	op.Info("asking targets", zap.Int("count", len(targets)))

	for _, t := range targets {
		fields := []zap.Field{
			zap.Int64("target_id", t.ID),
			zap.String("username", t.Username),
		}
		text := fmt.Sprintf("@%s %s", t.Username, s.question)
		msgID, err := s.sender.Send(s.chatID, text)
		if err != nil {
			op.Error("send question failed", append(fields, zap.Error(err))...)
			continue
		}
		if err := s.repo.RecordQuestion(ctx, t.ID, s.chatID, msgID); err != nil {
			op.Error("record question failed", append(fields, zap.Int64("message_id", msgID), zap.Error(err))...)
			continue
		}
		op.Debug("question sent", append(fields, zap.Int64("message_id", msgID))...)
	}
}

// HandleReply records an answer if the reply matches an open question.
func (s *Service) HandleReply(ctx context.Context, chatID, replyToMessageID, answerMessageID, telegramUserID int64, text string) {
	op := s.log.With(
		zap.String("op", "handle_reply"),
		zap.Int64("user_id", telegramUserID),
		zap.Int64("reply_to_message_id", replyToMessageID),
		zap.Int64("answer_message_id", answerMessageID),
	)
	matched, err := s.repo.RecordAnswer(ctx, chatID, replyToMessageID, answerMessageID, telegramUserID, text)
	if err != nil {
		op.Error("record answer failed", zap.Error(err))
		return
	}
	if matched {
		op.Info("answer recorded", zap.String("text", text))
	} else {
		op.Debug("reply did not match an open question")
	}
}

func (s *Service) SendStats(ctx context.Context, chatID int64) {
	op := s.log.With(zap.String("op", "send_stats"))
	stats, err := s.repo.Stats(ctx)
	if err != nil {
		op.Error("load stats failed", zap.Error(err))
		return
	}
	if len(stats) == 0 {
		if _, err := s.sender.Send(chatID, "Пока нет данных"); err != nil {
			op.Error("send empty stats failed", zap.Error(err))
		}
		return
	}
	var sb strings.Builder
	sb.WriteString("Статистика:\n")
	for _, st := range stats {
		fmt.Fprintf(&sb, "@%s — задано: %d, ответил: %d, проигнорил: %d\n",
			st.Username, st.Asked, st.Answered, st.Ignored)
	}
	if _, err := s.sender.Send(chatID, sb.String()); err != nil {
		op.Error("send stats failed", zap.Error(err))
		return
	}
	op.Debug("stats sent", zap.Int("rows", len(stats)))
}

func (s *Service) RunScheduler(ctx context.Context) {
	op := s.log.With(zap.String("op", "scheduler"), zap.Duration("interval", s.interval))
	op.Info("scheduler started")

	last, err := s.repo.LastAskedAt(ctx)
	if err != nil {
		op.Error("last asked lookup failed", zap.Error(err))
	}

	next := time.Now()
	if !last.IsZero() {
		next = last.Add(s.interval)
	}

	for {
		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		op.Debug("waiting for next round", zap.Time("next", next), zap.Duration("wait", wait))
		select {
		case <-ctx.Done():
			op.Info("scheduler stopping")
			return
		case <-time.After(wait):
			s.AskAll(ctx)
			next = time.Now().Add(s.interval)
		}
	}
}
