package limit

import (
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/time/rate"
)

func (ls *LimitSend) newLimit(path string) {
	ls.lock.Lock()
	ls.limit[path] = rate.NewLimiter(rate.Every(time.Second*60), 1)
	ls.lock.Unlock()
}

func (ls *LimitSend) get(path string) *rate.Limiter {
	ls.lock.Lock()
	defer ls.lock.Unlock()
	return ls.limit[path]
}

func (ls *LimitSend) deleteLimit(list []string) {
	ls.lock.Lock()
	for _, v := range list {
		delete(ls.limit, v)
	}
	ls.lock.Unlock()
}

type Option func(*LimitSend)

type LimitSend struct {
	LimitLen       int
	LimitGenSecond int
	Bucket         int
	l              log.Logger
	lock           sync.Mutex
	limit          map[string]*rate.Limiter
}

func defaultLimitSend() *LimitSend {
	return &LimitSend{
		LimitGenSecond: 60,
		Bucket:         1,
		l:              log.NewJSONLogger(os.Stdout),
	}
}
func WithEverySecond(second int) Option {
	return func(ls *LimitSend) {
		ls.LimitGenSecond = second
	}
}

func WithLog(l log.Logger) Option {
	return func(ls *LimitSend) {
		ls.l = l
	}
}
func WithBucket(number int) Option {
	return func(ls *LimitSend) {
		ls.Bucket = number
	}
}

type LimitInterface interface {
	LimitSend(filePath string, f func())
	RangeDelete(list []string)
}

func (ls *LimitSend) allow(path string) bool {
	return ls.get(path).Allow()
}

func NewLimit(limitLen int, opt ...Option) LimitInterface {
	o := defaultLimitSend()
	for _, v := range opt {
		v(o)
	}

	o.limit = make(map[string]*rate.Limiter, limitLen)
	o.lock = sync.Mutex{}
	return o
}
func (ls *LimitSend) LimitSend(filePath string, f func()) {
	if ls.get(filePath) == nil {
		ls.newLimit(filePath)
	}

	if ls.allow(filePath) {
		f()
	} else {
		level.Warn(ls.l).Log("filepath", filePath, "trigger", "limit send")
	}

}

func (ls *LimitSend) RangeDelete(list []string) {
	deleteList := make([]string, 0, len(ls.limit)/2)
	for k := range ls.limit {
		get := false
		for _, l := range list {
			if k == l {
				get = true
				break
			}
		}
		if !get {
			deleteList = append(deleteList, k)
		}
	}
	level.Info(ls.l).Log("clear range limit send", deleteList)
	ls.deleteLimit(deleteList)
}
