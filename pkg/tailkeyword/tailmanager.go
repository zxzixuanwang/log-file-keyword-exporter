package tailkeyword

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hpcloud/tail"
	"github.com/zxzixuanwang/log-file-keyword-exporter/conf"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/check"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/filter"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/resolve"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/savepostion"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/scan"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/limit"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/tsdb"
	"github.com/zxzixuanwang/log-file-keyword-exporter/tool"
)

var (
	TailChan = make(chan *TailWordIn)
	once     sync.Once
)

type tailManager struct {
	First bool
	l     log.Logger
	Ttl   int
	SP    *savepostion.SavePos
}

func NewTailManager(isFirst bool, l log.Logger, ttl int,
	tsdbAddress, tsdbUa string, tsdbTimeOut int,
	sp *savepostion.SavePos) *tailManager {

	tm := new(tailManager)
	tm.First = isFirst
	tm.l = l
	tm.Ttl = ttl

	tm.SP = sp
	return tm
}

func getRulerName(appName string) string {
	if len(conf.ConfigLogFile) > 0 {
		return conf.ConfigLogFile[appName].RulerName
	} else {
		return ""
	}
}
func (tm *tailManager) Reload(fileDirs []string) error {

	level.Info(tm.l).Log("reloading log", "...")

	result := tm.SP.Load(tm.First)
	level.Info(tm.l).Log("value", result)

	sc := scan.NewScan(&conf.AppConfig.LogFile.Ttl)
	fileTarget := sc.ScanDir(tm.l, &scan.FlushPolicy{
		FileDir: fileDirs,
	})
	nd := scan.NewDirs()
	level.Debug(tm.l).Log("older dir", nd.Get())
	newDir, sameDir, oldDir := sc.Dir(tm.First, fileTarget, nd.Get(), tm.l)
	nd.Set(append(newDir, sameDir...))

	level.Debug(tm.l).Log("old dir", oldDir, "same dir", sameDir, "new dir", newDir)
	for _, v := range oldDir {
		level.Debug(tm.l).Log("old dir", v)
		s := check.Get(v)

		tails, ok := s.(*tail.Tail)
		if !ok {
			level.Warn(tm.l).Log("cannot get conved tail,content is", s)
			continue
		}
		fileName := tails.Filename
		ctxAny := check.GetAndDelete(check.KeyCtx(fileName))
		ctxFunCancel, ok := ctxAny.(context.CancelFunc)
		if ok {
			level.Info(tm.l).Log("closing tailing signal，filename is", fileName)
			ctxFunCancel()
		}
		// 删除失效的文件名
		check.Delete([]string{tails.Filename})
		// 死亡再次缓存
		offset, err := tails.Tell()
		if err != nil {
			level.Error(tm.l).Log("take dying offset failed", err)
			continue
		}
		appName := check.Get(check.KeyAppName(fileName)).(string)

		fi := &savepostion.FIInput{
			FileName: fileName,
			Offset:   offset,
			AppName:  appName,
		}
		tm.SP.HotSave(fi)

		check.InsertWithClear(check.KeyDyingFile(fileName), offset)
	}

	for _, v := range newDir {
		level.Info(tm.l).Log("new dir", v)
		appName, offset, whence := getFileInfo(tm.l, result, v)
		level.Debug(tm.l).Log("appname", appName, "offset", offset, "whence", whence)

		keywords, ok := getAppKeyword(appName, tm.l)
		if !ok {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		TailChan <- &TailWordIn{
			FileName:  v,
			ReOpen:    true,
			Follow:    true,
			Offset:    offset,
			Whence:    whence,
			MustExist: false,
			Poll:      false,
			KeyWord:   keywords,
			AppName:   appName,
			Ctx:       ctx,
			RulerName: getRulerName(appName),
		}
		check.Insert(check.KeyCtx(v), cancel)

		tm.SP.HotSave(&savepostion.FIInput{
			FileName: v,
			Offset:   offset,
			AppName:  appName,
		})
	}

	for _, v := range sameDir {

		appName, offset, whence := getFileInfo(tm.l, result, v)

		level.Debug(tm.l).Log("appname", appName, "offset", offset, "whence", whence)
		if tm.First {

			keywords, ok := getAppKeyword(appName, tm.l)
			if !ok {
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			TailChan <- &TailWordIn{
				FileName:  v,
				ReOpen:    true,
				Follow:    true,
				Offset:    offset,
				Whence:    whence,
				MustExist: false,
				Poll:      false,
				KeyWord:   keywords,
				AppName:   appName,
				Ctx:       ctx,
				RulerName: getRulerName(appName),
			}

			check.Insert(check.KeyCtx(v), cancel)
		} else {
			s := check.Get(v)
			tails, ok := s.(*tail.Tail)
			if !ok {
				level.Warn(tm.l).Log("cannot get conved tail,content is", s)
				continue
			}
			offset, err := tails.Tell()
			if err != nil {
				level.Error(tm.l).Log("get offset failed, error", err)
				continue
			}

			level.Debug(tm.l).Log("filename", v, "offset", offset)
			fi := &savepostion.FIInput{
				Offset:   offset,
				AppName:  appName,
				FileName: tails.Filename,
			}
			tm.SP.HotSave(fi)
		}
		level.Debug(tm.l).Log("same dir", v)
	}
	once.Do(
		func() {
			tm.First = false
		})

	return nil
}

func (tm *tailManager) Do(rso resolve.ResolveInterface, pro tsdb.PromRemoteInterface, limit limit.LimitInterface) {
	go func() {
		for v := range TailChan {
			v := v
			level.Debug(tm.l).Log("ranger", v)
			ntwi := NewTailWordInfo(&TailWordInfo{
				L:       tm.l,
				Minute:  0,
				Buff:    0,
				Pro:     pro,
				Limit:   limit,
				Resolve: rso,
			})
			go ntwi.TailWord(v, v.Ctx, filter.NewFilter(filter.DefaultFilter{}).HaveFilter)
		}
	}()

}
func getAppKeyword(name string, l log.Logger) ([]string, bool) {
	keywordsInterface := check.Get(name)
	keywords, ok := keywordsInterface.([]string)
	if !ok {
		level.Error(l).Log("conv app keyword tailed, appname is", name)
	}
	return keywords, ok
}

func getFileInfo(l log.Logger, result map[string]*savepostion.FileInfo, v string) (appName string, offset int64, whence int) {

	// 增加获取死亡的内容
	newOffset, ok := check.GetAndDelete(check.KeyDyingFile(v)).(int64)
	if ok {
		level.Info(l).Log("get died info offset", newOffset)

		offset = newOffset
	}

	appName = check.Get(check.KeyAppName(v)).(string)
	if result[appName] != nil {

		offset = tool.MaxNumber(offset, result[appName].Offset)
		whence = 0

	}
	return
}
