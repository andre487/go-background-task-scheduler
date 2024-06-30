package bgscheduler

import (
	"database/sql"
	"log"
)

type ExactLaunchTime struct {
	Hour   int
	Minute int
	Second int
}

func (r *ExactLaunchTime) Equals(other ExactLaunchTime) bool {
	return other.Hour == r.Hour && other.Minute == r.Minute && other.Second == r.Second
}

type LogLevel uint32

// Same uint32 values that in Logrus
//
//goland:noinspection GoUnusedConst
const (
	LogLevelPanic LogLevel = iota
	LogLevelFatal
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
)

func (t LogLevel) String() string {
	switch t {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	default:
		return "ERROR"
	}
}

type Logger interface {
	Printf(format string, v ...any)
}

type Config struct {
	Logger   Logger
	LogLevel LogLevel
	DbPath   string
}

type Scheduler struct {
	logger *logWrap
	db     *sql.DB
}

func NewBgScheduler(conf *Config) (*Scheduler, error) {
	if conf.Logger == nil {
		conf.Logger = log.Default()
	}

	t := Scheduler{
		logger: newLogWrap(conf.Logger, conf.LogLevel),
	}

	return &t, nil
}
