package logger

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type gormZap struct {
	base          *zap.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGorm wraps a *zap.Logger as a gorm logger, surfacing SQL traces through
// the same structured logging pipeline as the rest of the app.
func NewGorm(z *zap.Logger, level gormlogger.LogLevel) gormlogger.Interface {
	return &gormZap{
		base:          z.With(zap.String("component", "gorm")),
		level:         level,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (g *gormZap) LogMode(l gormlogger.LogLevel) gormlogger.Interface {
	cp := *g
	cp.level = l
	return &cp
}

func (g *gormZap) Info(_ context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Info {
		g.base.Sugar().Infof(msg, args...)
	}
}

func (g *gormZap) Warn(_ context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Warn {
		g.base.Sugar().Warnf(msg, args...)
	}
}

func (g *gormZap) Error(_ context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Error {
		g.base.Sugar().Errorf(msg, args...)
	}
}

func (g *gormZap) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && g.level >= gormlogger.Error:
		g.base.Error("query failed", append(fields, zap.Error(err))...)
	case g.slowThreshold > 0 && elapsed > g.slowThreshold && g.level >= gormlogger.Warn:
		g.base.Warn("slow query", fields...)
	case g.level >= gormlogger.Info:
		g.base.Debug("query", fields...)
	}
}
