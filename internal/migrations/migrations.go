package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Run applies all pending migrations in order.
func Run(db *gorm.DB, log *zap.Logger) error {
	log = log.With(zap.String("component", "migrations"))
	migs := all()
	log.Info("running migrations", zap.Int("count", len(migs)))

	m := gormigrate.New(db, gormigrate.DefaultOptions, migs)
	if err := m.Migrate(); err != nil {
		log.Error("migration failed", zap.Error(err))
		return err
	}
	log.Info("migrations up to date")
	return nil
}

// Rollback rolls back the most recently applied migration.
func Rollback(db *gorm.DB, log *zap.Logger) error {
	log = log.With(zap.String("component", "migrations"))
	m := gormigrate.New(db, gormigrate.DefaultOptions, all())
	if err := m.RollbackLast(); err != nil {
		log.Error("rollback failed", zap.Error(err))
		return err
	}
	log.Info("rolled back last migration")
	return nil
}

func all() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		initSchema(),
		askLimits(),
	}
}
