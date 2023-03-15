package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hpcloud/tail"
	"github.com/zxzixuanwang/log-file-keyword-exporter/conf"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/check"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/resolve"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/savepostion"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/scan"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/limit"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/logbean"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/tailkeyword"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/tsdb"
	"github.com/zxzixuanwang/log-file-keyword-exporter/tool"
)

var l log.Logger

func main() {
	runtime.GOMAXPROCS(conf.AppConfig.App.CpuNumber)

	ip, err := tool.GetIP()
	if err != nil {
		level.Error(l).Log("get ip error", err)
		panic(err)
	}

	conf.Ip = ip.String()

	signalChan := make(chan os.Signal, 1)
	l = logbean.GetLog(logbean.NewLogInfo(
		conf.AppConfig.Log.FilePosition,
		conf.AppConfig.Log.Level))

	fileDirs := make([]string, 0, len(conf.AppConfig.LogFile.List))
	appNames := make([]string, 0, len(conf.AppConfig.LogFile.List))

	for _, v := range conf.AppConfig.LogFile.List {
		fileDirs = append(fileDirs, v.FilePosition)
		check.Insert(check.KeyAppName(v.FilePosition), v.AppName)
		if len(v.ResolveKeyWord) > 0 {
			appNames = append(appNames, v.AppName)
		}
	}

	if err := tool.CreateDir(conf.AppConfig.Log.FilePosition); err != nil {
		panic(err)
	}

	fileTarget := scan.NewScan(&conf.AppConfig.LogFile.Ttl).ScanDir(l, &scan.FlushPolicy{
		FileDir: fileDirs,
	})

	level.Debug(l).Log("filterd dirs", fileTarget)
	nd := scan.NewDirs()
	nd.Set(fileTarget)

	sp := savepostion.NewSavePos(conf.AppConfig.LogFile.PositionDir, l)

	for _, v := range conf.AppConfig.LogFile.List {
		check.Insert(v.AppName, v.KeyWords)

		level.Info(l).Log("app", v.AppName, "keyword", v.KeyWords)
	}

	ri := resolve.NewResolver(
		resolve.WithAppName(appNames),
	)

	ntl := tailkeyword.NewTailManager(true, l,
		conf.AppConfig.LogFile.Save,
		conf.AppConfig.Tsdb.Address,
		conf.AppConfig.Tsdb.UserAgent,
		conf.AppConfig.Tsdb.TimeOut,
		sp)

	npr, err := tsdb.NewProRemote(
		tsdb.WithIpAddress(conf.AppConfig.Tsdb.Address),
		tsdb.WithTimeOut(conf.AppConfig.Tsdb.TimeOut),
		tsdb.WithLog(l),
	)

	if err != nil {
		level.Error(l).Log("create prome write instance failed", err)
		panic(err)
	}

	lim := limit.NewLimit(
		len(conf.AppConfig.LogFile.List)*2,
		limit.WithBucket(conf.AppConfig.Tsdb.Bucket),
		limit.WithEverySecond(conf.AppConfig.Tsdb.RateGen),
		limit.WithLog(l),
	)

	ntl.Do(ri, npr, lim)
	ntl.Reload(fileDirs)

	signal.Notify(signalChan,
		os.Interrupt,
		syscall.SIGALRM,
		syscall.SIGHUP,
		// syscall.SIGINFO, this causes windows to fail
		syscall.SIGINT,
		// syscall.SIGQUIT, // Quit from keyboard, "kill -3"
	)

	if conf.AppConfig.App.Debug {

		level.Info(l).Log("starting listen pprof,port", conf.AppConfig.App.Port)
		go func() {
			level.Error(l).Log((http.ListenAndServe(conf.AppConfig.App.Port, nil)))
		}()
	}
	if conf.AppConfig.LogFile != nil {
		if err := tool.CreateDir(conf.AppConfig.Log.FilePosition); err != nil {
			panic(err)
		}
		saveFileTime := time.NewTicker(time.Minute * time.Duration(conf.AppConfig.LogFile.Save))
		scanFileTime := time.NewTicker(time.Minute * time.Duration(conf.AppConfig.LogFile.Check))
		clearMap := time.NewTicker(time.Hour)
		clearLimit := time.NewTicker(time.Hour)
		resend := time.NewTicker(time.Minute)
		for {
			select {
			case <-signalChan:
				level.Info(l).Log("saving tell info", "...")
				savePostionInFile(sp, true)
				level.Info(l).Log("closing", "...")
				os.Exit(1)

			case <-saveFileTime.C:
				level.Info(l).Log("saving tell info", "position")
				savePostionInFile(sp, false)
			case <-scanFileTime.C:
				if err := ntl.Reload(fileDirs); err != nil {
					level.Error(l).Log("reload dir err ", err)
				}

			case <-clearMap.C:
				level.Info(l).Log("clear fileInfoMap", "内容")
				check.ExpireDelete(l)
			case <-clearLimit.C:
				dirs := nd.Get()
				level.Info(l).Log("clear dir limit in rate ", dirs)
				lim.RangeDelete(dirs)
			case <-resend.C:
				ri.Range(func(appName string) {
					npr.Send(float64(1),
						tsdb.NewPromLabels(appName, savepostion.Get(appName).FileName,
							conf.Ip,
							tsdb.WithOthers(map[string][]string{"keywords": conf.ConfigLogFile[appName].KeyWords,
								"rulerName": {conf.ConfigLogFile[appName].RulerName}}),
						).
							GenLabels(),
					)
				})
			}
		}
	}
}
func savePostionInFile(sp *savepostion.SavePos, kill bool) {
	level.Debug(l).Log("saving", "position")
	fis := make([]*savepostion.FIInput, 0, 20)
	check.Range(func(k string, v any) {
		ta, ok := v.(*tail.Tail)

		level.Debug(l).Log("range map", ta)
		if ok {

			offset, err := ta.Tell()
			appName := check.Get(check.KeyAppName(k)).(string)
			fileName := ta.Filename
			level.Debug(l).Log("appname", appName, "filename", fileName)

			fi := savepostion.Get(appName)

			if fi != nil {
				level.Info(l).Log("get offset in file,offset is", fi.Offset, "filename is", fi.FileName)
				if offset < fi.Offset {
					offset = fi.Offset
				}
			}
			if err != nil {
				level.Warn(l).Log("offset error", err, "getting file ", "content")
			}

			level.Info(l).Log("getting current offset", offset, "filename", fileName)
			fis = append(fis, &savepostion.FIInput{
				FileName: fileName,
				Offset:   offset,
				AppName:  appName,
			})

		}
	})
	if len(fis) > 0 {
		sp.PatchSave(fis)
	}
}
