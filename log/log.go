package log

import (
	"sync"

	"go.uber.org/zap"
)

var (
	logger *zap.Logger
	once   sync.Once
)

func instance() *zap.Logger {
	var err error
	once.Do(func() {
		logger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
		defer logger.Sync()
	})
	return logger
}

func Logger() *zap.Logger {
	return instance()
}
