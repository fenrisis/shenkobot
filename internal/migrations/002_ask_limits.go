package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Frozen schema models for migration 002. Do NOT edit after release.

// askerV1 - пользователи которые используют команду /ask (не таргеты)
type askerV1 struct {
	ID         int64     `gorm:"primaryKey"`
	TelegramID int64     `gorm:"uniqueIndex;not null"`
	Username   string    `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
	UpdatedAt  time.Time `gorm:"not null;default:now()"`
}

func (askerV1) TableName() string { return "askers" }

// askUsageV1 - история использования команды /ask для лимитов
type askUsageV1 struct {
	ID       int64     `gorm:"primaryKey"`
	AskerID  int64     `gorm:"index;not null"`
	ChatID   int64     `gorm:"not null;index:idx_ask_usage_chat,priority:1"`
	AskedAt  time.Time `gorm:"not null;default:now();index:idx_ask_usage_chat,priority:2"`
}

func (askUsageV1) TableName() string { return "ask_usage" }

// settingV1 - настройки приложения (вместо env переменных)
type settingV1 struct {
	Key         string `gorm:"primaryKey"`
	Value       string `gorm:"not null"`
	Description string
}

func (settingV1) TableName() string { return "settings" }

func askLimits() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260429_002_ask_limits",
		Migrate: func(tx *gorm.DB) error {
			// Создать таблицы
			if err := tx.Migrator().CreateTable(&askerV1{}, &askUsageV1{}, &settingV1{}); err != nil {
				return err
			}

			// Добавить FK constraint
			if err := tx.Exec(`
				ALTER TABLE ask_usage
				ADD CONSTRAINT fk_ask_usage_asker
				FOREIGN KEY (asker_id) REFERENCES askers(id) ON DELETE CASCADE
			`).Error; err != nil {
				return err
			}

			// Добавить индекс для быстрого поиска последнего global ask
			if err := tx.Exec(`
				CREATE INDEX idx_ask_usage_lookup ON ask_usage(chat_id, asked_at DESC)
			`).Error; err != nil {
				return err
			}

			// Добавить начальные настройки
			settings := []settingV1{
				{
					Key:         "global_cooldown_hours",
					Value:       "3",
					Description: "Глобальный кулдаун между /ask в чате (часы)",
				},
				{
					Key:         "personal_limit_per_day",
					Value:       "3",
					Description: "Персональный лимит /ask на юзера в день",
				},
			}

			for _, s := range settings {
				if err := tx.Create(&s).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&askUsageV1{}, &askerV1{}, &settingV1{})
		},
	}
}
