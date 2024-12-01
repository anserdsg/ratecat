package logging

import (
	"io"
	"strings"

	"github.com/shaj13/raft/raftlog"
	prettyconsole "github.com/thessem/zap-prettyconsole"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type raftLogger struct {
	*zap.SugaredLogger
	v *verbose
}

func newRaftNopLogger() *raftLogger {
	logger := &raftLogger{
		zap.New(zapcore.NewNopCore()).Sugar(),
		nil,
	}
	logger.v = &verbose{logger: logger, lv: FatalLevel}
	return logger
}

func newRaftLogger(writer io.Writer, level zapcore.Level, env string, extraOpts ...zap.Option) *raftLogger {
	var core zapcore.Core
	opts := make([]zap.Option, 0, len(extraOpts))
	switch env {
	case "production", "benchmark":
		core = zapcore.NewNopCore()

	case "development", "mock":
		cfg := prettyconsole.NewEncoderConfig()
		cfg.CallerKey = "C"
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncodeTime = prettyconsole.DefaultTimeEncoder("15:04:05.000")
		opts = append(opts, zap.WithCaller(false), zap.AddStacktrace(zapcore.ErrorLevel))
		core = zapcore.NewCore(
			prettyconsole.NewEncoder(cfg),
			zapcore.Lock(zapcore.AddSync(writer)),
			level,
		)

	case "test":
		cfg := zapcore.EncoderConfig{
			MessageKey:     "msg",
			LevelKey:       "level",
			NameKey:        "logger",
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		}
		opts = append(opts, zap.WithCaller(true))
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg),
			zapcore.Lock(zapcore.AddSync(writer)),
			level,
		)
	}
	opts = append(opts, extraOpts...)

	logger := &raftLogger{
		zap.New(core, opts...).Sugar(),
		nil,
	}
	logger.v = &verbose{logger: logger, lv: level}
	return logger
}

func (l *raftLogger) V(lv int) raftlog.Verbose {
	return verbose{logger: l, lv: zapcore.Level(lv)}
}

func (l *raftLogger) Warning(v ...any) {
	l.Warn(v...)
}

func (l *raftLogger) Warningf(format string, v ...any) {
	l.Warnf(format, v...)
}

func (l *raftLogger) Errorf(format string, v ...any) {
	if strings.HasPrefix(format, "raft.membership: sending message to member") {
		return
	}
	l.SugaredLogger.Errorf(format, v...)
}

type verbose struct {
	logger *raftLogger
	lv     zapcore.Level
}

func (ver verbose) Enabled() bool {
	return true
}

func (ver verbose) Error(v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Error(v...)
	}
}

func (ver verbose) Errorf(format string, v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Errorf(format, v...)
	}
}

func (ver verbose) Warning(v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Warn(v...)
	}
}

func (ver verbose) Warningf(format string, v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Warnf(format, v...)
	}
}

func (ver verbose) Info(v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Info(v...)
	}
}

func (ver verbose) Infof(format string, v ...interface{}) {
	if ver.Enabled() {
		ver.logger.Infof(format, v...)
	}
}
