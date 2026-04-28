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
