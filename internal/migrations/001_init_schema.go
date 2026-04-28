package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Frozen schema models for migration 001. Do NOT edit after release —
// schema changes must land in a new migration file with new frozen models.

type targetV1 struct {
	ID         int64  `gorm:"primaryKey"`
	TelegramID int64  `gorm:"uniqueIndex;not null"`
	Username   string `gorm:"not null"`
}

func (targetV1) TableName() string { return "targets" }

type questionV1 struct {
	ID              int64     `gorm:"primaryKey"`
	TargetID        int64     `gorm:"index;not null"`
	ChatID          int64     `gorm:"not null;index:idx_questions_lookup,priority:1"`
	MessageID       int64     `gorm:"not null;index:idx_questions_lookup,priority:2"`
	AskedAt         time.Time `gorm:"not null;default:now()"`
	AnsweredAt      *time.Time
	AnswerText      *string
	AnswerMessageID *int64
}

func (questionV1) TableName() string { return "questions" }

func initSchema() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260428_001_init_schema",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().CreateTable(&targetV1{}, &questionV1{}); err != nil {
				return err
			}
			return tx.Exec(`
				ALTER TABLE questions
				ADD CONSTRAINT fk_questions_target
				FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&questionV1{}, &targetV1{})
		},
	}
}
