package repository

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Target struct {
	ID         int64  `gorm:"primaryKey"`
	TelegramID int64  `gorm:"uniqueIndex;not null"`
	Username   string `gorm:"not null"`
}

type Question struct {
	ID              int64     `gorm:"primaryKey"`
	TargetID        int64     `gorm:"index;not null"`
	ChatID          int64     `gorm:"not null;index:idx_questions_lookup,priority:1"`
	MessageID       int64     `gorm:"not null;index:idx_questions_lookup,priority:2"`
	AskedAt         time.Time `gorm:"not null;default:now()"`
	AnsweredAt      *time.Time
	AnswerText      *string
	AnswerMessageID *int64
}

type Asker struct {
	ID         int64     `gorm:"primaryKey"`
	TelegramID int64     `gorm:"uniqueIndex;not null"`
	Username   string    `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
	UpdatedAt  time.Time `gorm:"not null;default:now()"`
}

type AskUsage struct {
	ID      int64     `gorm:"primaryKey"`
	AskerID int64     `gorm:"index;not null"`
	ChatID  int64     `gorm:"not null;index:idx_ask_usage_chat,priority:1"`
	AskedAt time.Time `gorm:"not null;default:now();index:idx_ask_usage_chat,priority:2"`
}

type Setting struct {
	Key         string `gorm:"primaryKey"`
	Value       string `gorm:"not null"`
	Description string
}

type Stat struct {
	Username string
	Asked    int
	Answered int
	Ignored  int
}

type Repository struct {
	db *gorm.DB
}

// New opens a Postgres connection pool. gormLog is wired into gorm.Config so
// SQL traces / slow-query warnings flow through the same structured logger
// as the rest of the app. Pass nil to use gorm's default logger.
func New(url string, gormLog gormlogger.Interface) (*Repository, error) {
	cfg := &gorm.Config{}
	if gormLog != nil {
		cfg.Logger = gormLog
	}
	db, err := gorm.Open(postgres.Open(url), cfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// DB exposes the underlying *gorm.DB so the migrations runner can use it
// without coupling the migrations package to the repository layer.
func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (r *Repository) Targets(ctx context.Context) ([]Target, error) {
	var out []Target
	err := r.db.WithContext(ctx).Order("id").Find(&out).Error
	return out, err
}

func (r *Repository) RecordQuestion(ctx context.Context, targetID, chatID, messageID int64) error {
	return r.db.WithContext(ctx).Create(&Question{
		TargetID:  targetID,
		ChatID:    chatID,
		MessageID: messageID,
	}).Error
}

// RecordAnswer matches a reply to an open question for the given user and stores it.
// Returns true if a question was matched.
func (r *Repository) RecordAnswer(ctx context.Context, chatID, replyToMessageID, answerMessageID, telegramUserID int64, text string) (bool, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&Question{}).
		Where(`chat_id = ? AND message_id = ? AND answered_at IS NULL
		       AND target_id IN (SELECT id FROM targets WHERE telegram_id = ?)`,
			chatID, replyToMessageID, telegramUserID).
		Updates(map[string]any{
			"answered_at":       &now,
			"answer_text":       text,
			"answer_message_id": answerMessageID,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *Repository) Stats(ctx context.Context) ([]Stat, error) {
	var out []Stat
	err := r.db.WithContext(ctx).
		Table("targets t").
		Select(`t.username,
		        COUNT(q.id)                          AS asked,
		        COUNT(q.answered_at)                 AS answered,
		        COUNT(q.id) - COUNT(q.answered_at)   AS ignored`).
		Joins("LEFT JOIN questions q ON q.target_id = t.id").
		Group("t.username").
		Order("t.username").
		Scan(&out).Error
	return out, err
}

func (r *Repository) LastAskedAt(ctx context.Context) (time.Time, error) {
	var t sql.NullTime
	err := r.db.WithContext(ctx).
		Model(&Question{}).
		Select("MAX(asked_at)").
		Scan(&t).Error
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}

// GetSetting получает значение настройки по ключу
func (r *Repository) GetSetting(ctx context.Context, key string) (string, error) {
	var setting Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// IsTarget проверяет что пользователь является таргетом
func (r *Repository) IsTarget(ctx context.Context, telegramID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Target{}).
		Where("telegram_id = ?", telegramID).
		Count(&count).Error
	return count > 0, err
}

// GetOrCreateAsker получает или создаёт запись asker
func (r *Repository) GetOrCreateAsker(ctx context.Context, telegramID int64, username string) (*Asker, error) {
	var asker Asker
	err := r.db.WithContext(ctx).
		Where("telegram_id = ?", telegramID).
		First(&asker).Error

	if err == nil {
		// Asker найден, обновляем username если изменился
		if asker.Username != username {
			asker.Username = username
			asker.UpdatedAt = time.Now()
			if err := r.db.WithContext(ctx).Save(&asker).Error; err != nil {
				return nil, err
			}
		}
		return &asker, nil
	}

	// Если не найден, создаём нового
	if err == gorm.ErrRecordNotFound {
		asker = Asker{
			TelegramID: telegramID,
			Username:   username,
		}
		if err := r.db.WithContext(ctx).Create(&asker).Error; err != nil {
			return nil, err
		}
		return &asker, nil
	}

	return nil, err
}

// RecordAskUsage записывает использование команды /ask
func (r *Repository) RecordAskUsage(ctx context.Context, askerID, chatID int64) error {
	usage := AskUsage{
		AskerID: askerID,
		ChatID:  chatID,
		AskedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Create(&usage).Error
}

// GetLastGlobalAsk возвращает последнее использование /ask в чате с данными asker'а
func (r *Repository) GetLastGlobalAsk(ctx context.Context, chatID int64) (*AskUsage, *Asker, error) {
	var usage AskUsage
	err := r.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("asked_at DESC").
		First(&usage).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var asker Asker
	err = r.db.WithContext(ctx).First(&asker, usage.AskerID).Error
	if err != nil {
		return nil, nil, err
	}

	return &usage, &asker, nil
}

// GetTodayAskCount возвращает количество использований /ask за сегодня (с 00:00 UTC)
func (r *Repository) GetTodayAskCount(ctx context.Context, askerID int64) (int64, error) {
	// Начало сегодняшнего дня в UTC
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var count int64
	err := r.db.WithContext(ctx).
		Model(&AskUsage{}).
		Where("asker_id = ? AND asked_at >= ?", askerID, startOfDay).
		Count(&count).Error

	return count, err
}
