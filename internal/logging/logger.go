package logging

import (
	"os"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"go.uber.org/zap"
)

var (
	defLogger       *zap.SugaredLogger
	defLevelEnabler zap.AtomicLevel
	defRaftLogger   *raftLogger
)

func Init() *zap.SugaredLogger {
	logLevel := zapLogLevel(config.GetDefault().LogLevel)
	initDefLogger(os.Stderr, logLevel, config.Default.Env)

	return defLogger
}

func Default() *zap.SugaredLogger {
	return defLogger
}

func SetLevel(level LogLevel) {
	defLevelEnabler.SetLevel(level)
}

func RaftLogger(enabled bool) *raftLogger {
	if !enabled {
		return newRaftNopLogger()
	}
	if defRaftLogger == nil {
		logLevel := zapLogLevel(config.GetDefault().LogLevel)
		defRaftLogger = newRaftLogger(os.Stderr, logLevel, config.Default.Env)
	}

	return defRaftLogger
}
