package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Init initializes the global logger
func Init(serviceName string, env string) {
	config := zap.NewProductionConfig()
	config.Encoding = "json"

	// Change log level based on environment
	if strings.ToLower(env) == "dev" {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.EncoderConfig.TimeKey = "time"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.NameKey = "logger"
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.OutputPaths = []string{"stdout"}
	} else {
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.NameKey = "service"
		config.OutputPaths = []string{"stdout"}
	}

	var err error
	log, err = config.Build()
	if err != nil {
		panic(err)
	}

	log = log.With(zap.String("service", serviceName))
	zap.ReplaceGlobals(log)
}

// L returns the global zap logger instance
func L() *zap.Logger {
	if log == nil {
		// fallback logger
		fallback, _ := zap.NewProduction()
		return fallback
	}
	return log
}

// Sync flushes the logger
func Sync() {
	_ = log.Sync()
}
