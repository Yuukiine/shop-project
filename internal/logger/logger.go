package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func NewLogger(env string) *zap.Logger {
	logger, _ := zap.NewDevelopment()
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("[02-01-2006 15:04:05]")

	switch env {
	case envLocal:
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case envDev:
		logger, _ = zap.NewProduction()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case envProd:
		logger, _ = zap.NewProduction()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, _ = config.Build()

	return logger
}
