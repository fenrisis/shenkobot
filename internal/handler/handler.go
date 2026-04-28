package handler

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"shenkobot/internal/service"
)

type Handler struct {
	api    *tgbotapi.BotAPI
	svc    *service.Service
	chatID int64
	log    *zap.Logger
}

func New(api *tgbotapi.BotAPI, svc *service.Service, chatID int64, log *zap.Logger) *Handler {
	return &Handler{
		api:    api,
		svc:    svc,
		chatID: chatID,
		log:    log.With(zap.String("component", "handler"), zap.Int64("allowed_chat_id", chatID)),
	}
}

// TelegramSender adapts *tgbotapi.BotAPI to service.Sender.
type TelegramSender struct {
	api *tgbotapi.BotAPI
	log *zap.Logger
}

func NewTelegramSender(api *tgbotapi.BotAPI, log *zap.Logger) *TelegramSender {
	return &TelegramSender{
		api: api,
		log: log.With(zap.String("component", "sender")),
	}
}

func (s *TelegramSender) Send(chatID int64, text string) (int64, error) {
	msg, err := s.api.Send(tgbotapi.NewMessage(chatID, text))
	if err != nil {
		s.log.Error("telegram send failed", zap.Int64("chat_id", chatID), zap.Error(err))
		return 0, err
	}
	s.log.Debug("telegram send", zap.Int64("chat_id", chatID), zap.Int("message_id", msg.MessageID))
	return int64(msg.MessageID), nil
}

func (h *Handler) Run(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := h.api.GetUpdatesChan(u)

	h.log.Info("update loop started")
	for {
		select {
		case <-ctx.Done():
			h.api.StopReceivingUpdates()
			h.log.Info("update loop stopping")
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			if upd.Message == nil {
				continue
			}
			h.routeMessage(ctx, upd.Message)
		}
	}
}

func (h *Handler) routeMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.Chat.ID != h.chatID {
		fromID := int64(0)
		fromUsername := ""
		if msg.From != nil {
			fromID = msg.From.ID
			fromUsername = msg.From.UserName
		}
		h.log.Warn("dropped message from unauthorized chat",
			zap.Int64("chat_id", msg.Chat.ID),
			zap.String("chat_title", msg.Chat.Title),
			zap.Int64("from_user_id", fromID),
			zap.String("from_username", fromUsername),
			zap.Int("message_id", msg.MessageID),
		)
		return
	}
	h.handleMessage(ctx, msg)
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.IsCommand() {
		cmd := msg.Command()
		h.log.Debug("command received", zap.String("command", cmd), zap.Int64("from_user_id", fromID(msg)))
		switch cmd {
		case "stats":
			h.svc.SendStats(ctx, msg.Chat.ID)
		case "ask":
			h.svc.AskAll(ctx)
		default:
			h.log.Debug("unknown command ignored", zap.String("command", cmd))
		}
		return
	}

	if msg.ReplyToMessage == nil || msg.From == nil {
		return
	}
	if msg.ReplyToMessage.From == nil || msg.ReplyToMessage.From.ID != h.api.Self.ID {
		return
	}

	h.svc.HandleReply(ctx,
		msg.Chat.ID,
		int64(msg.ReplyToMessage.MessageID),
		int64(msg.MessageID),
		msg.From.ID,
		msg.Text,
	)
}

func fromID(m *tgbotapi.Message) int64 {
	if m.From == nil {
		return 0
	}
	return m.From.ID
}
