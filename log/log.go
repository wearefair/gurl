package log

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	once   sync.Once
)

// Create instance of logger with colorized logging and stack trace disabled
func instance() *zap.Logger {
	var err error
	once.Do(func() {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.DisableStacktrace = true
		logger, err = config.Build()
		if err != nil {
			panic(err)
		}
		defer logger.Sync()
	})
	return logger
}

// Logger returns the instance of logger
func Logger() *zap.Logger {
	return instance()
}

// WrapError is a helper func to log the error passed in and also return the err
func WrapError(err error) error {
	if err != nil {
		Logger().Error(err.Error())
	}
	return err
}
