package logbean

import (
	"os"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	l    log.Logger
	once sync.Once
)

type LogInfo struct {
	FilePosition string
	Level        string
}

type Options func(*LogInfo)

func WithFilePostion(position string) Options {
	return func(li *LogInfo) {
		li.FilePosition = position
	}
}

func WithLevel(lev string) Options {
	return func(li *LogInfo) {
		li.Level = lev
	}
}

func NewLogInfo(filePosition, lev string) Options {
	return func(li *LogInfo) {
		li.FilePosition = filePosition
		li.Level = lev
	}
}

func GetLog(opt ...Options) log.Logger {
	li := defaultInfo()
	for _, o := range opt {
		o(li)
	}
	once.Do(func() {

		f, _ := os.OpenFile(li.FilePosition, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		logger := log.NewJSONLogger(f)
		logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.Caller(5))
		if li.Level == "all" {
			logger = level.NewFilter(logger, level.AllowAll())
		} else {
			logger = level.NewFilter(logger, level.Allow(logLevelFilter(li.Level)))
		}
		l = logger
	})
	return l
}

func defaultInfo() *LogInfo {
	return &LogInfo{
		FilePosition: "./app.log",
		Level:        "info",
	}
}

func logLevelFilter(lev string) level.Value {
	switch lev {
	case "debug":
		return level.DebugValue()
	case "error":
		return level.ErrorValue()
	case "warn":
		return level.WarnValue()
	default:
		return level.InfoValue()
	}
}
