package check

import (
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/zxzixuanwang/log-file-keyword-exporter/conf"
)

var (
	fileInfoMap = make(map[string]any, 30*len(conf.AppConfig.LogFile.List))
	l           = sync.RWMutex{}
)

func Insert[T any](key string, value T) {
	l.Lock()
	defer l.Unlock()

	fileInfoMap[key] = value
}

func KeyAppName(fileName string) string {
	return fileName + "_appname"
}

func KeyDyingFile(fileName string) string {
	return fileName + "_dying"
}

func KeyCtx(name string) string {
	return name + "_ctx"
}

func InsertWithClear[T any](key string, value T) {
	fileInfoMap[strconv.FormatInt(time.Now().UnixNano(), 32)] = value
	fileInfoMap[key] = value
}
func Get(key string) any {
	l.RLock()
	defer l.RUnlock()
	return fileInfoMap[key]
}

func Delete(key []string) {
	l.Lock()
	defer l.Unlock()
	for _, v := range key {
		delete(fileInfoMap, v)
	}
}

func GetAndDelete(key string) any {
	t := fileInfoMap[key]
	l.Lock()
	delete(fileInfoMap, key)
	l.Unlock()
	return t
}

func ExpireDelete(l log.Logger) {
	t := time.Now().UnixNano()

	for k, v := range fileInfoMap {
		compare, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			level.Info(l).Log("parse conv", err, "compared number", k)
			continue
		}
		if compare < t {
			level.Info(l).Log("delete key", k, "delete value", v)
			Delete([]string{k, v.(string)})
		}
	}
}
func Range(f func(k string, v any)) {
	for k, v := range fileInfoMap {
		f(k, v)
	}
}
