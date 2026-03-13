package observabilidade

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NovoLogger(nivel string) *zap.Logger {
	var zapNivel zapcore.Level
	switch nivel {
	case "debug":
		zapNivel = zapcore.DebugLevel
	case "warn":
		zapNivel = zapcore.WarnLevel
	case "error":
		zapNivel = zapcore.ErrorLevel
	default:
		zapNivel = zapcore.InfoLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapNivel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, _ := cfg.Build()
	return logger
}
