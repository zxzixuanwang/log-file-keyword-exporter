package scan

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/check"
	"github.com/zxzixuanwang/log-file-keyword-exporter/tool"
)

func checkDir(l log.Logger, expectTime time.Time, fileDirs map[string]string, f func(f *os.File, lastTime time.Time) (time.Time, bool)) []string {
	dirs := make([]string, 0, 1)
	checkDirs := make([]string, 0, len(fileDirs))
	var (
		lastTime time.Time
		open     bool
	)

	lastTime = expectTime
	for k := range fileDirs {
		file, err := os.Open(k)
		if err != nil {
			level.Error(l).Log("open file error", err, "打开失败文件", k)
			continue
		}
		defer file.Close()

		expectTime, open = f(file, lastTime)
		if open {
			checkDirs = append(checkDirs, k)
			lastTime = expectTime
		}
	}
	if len(checkDirs) > 0 {
		dir := checkDirs[len(checkDirs)-1]
		dirs = append(dirs, dir)

		if check.Get(dir) == nil {
			// insert dir_key = appname
			check.InsertWithClear(check.KeyAppName(dir), fileDirs[dir])
		}
	}
	return dirs
}

type FlushPolicy struct {
	FileDir []string
}

type scan struct {
	ExpectDuration int `json:"expectDuration,omitempty"`
}

func NewScan(expectDuration *int) *scan {
	if expectDuration == nil {
		expectDuration = tool.ChangeToPoint(5)
	}
	return &scan{
		ExpectDuration: *expectDuration,
	}
}

func (s *scan) ScanDir(l log.Logger, flush *FlushPolicy) []string {
	expectTime := time.Now().Add(-time.Minute * time.Duration(s.ExpectDuration))
	result := make(map[string]string, len(flush.FileDir))
	for _, v := range flush.FileDir {
		appName := check.Get(check.KeyAppName(v))
		if strings.Contains(v, "*") {
			matchs, err := filepath.Glob(v)
			if err != nil {
				level.Error(l).Log("get log dir failed,err", err, "path", v)
				continue
			}
			for _, m := range matchs {
				result[m] = appName.(string)
			}
		} else {
			result[v] = appName.(string)
		}
	}
	level.Debug(l).Log("scaning dir", result)
	return checkDir(l, expectTime, result, func(f *os.File, lastTime time.Time) (time.Time, bool) {
		return fileCheck(f, lastTime, l)
	})
}

func fileCheck(f *os.File, expectTime time.Time, l log.Logger) (time.Time, bool) {
	info, err := f.Stat()
	if err != nil {
		level.Error(l).Log("check file failed,err", err, "filename is", f.Name())
		return time.Time{}, false
	}
	level.Info(l).Log("filename", info.Name(), "modify time", info.ModTime(), "expect time ", expectTime)
	if info.ModTime().After(expectTime) {

		return info.ModTime(), true
	}
	return time.Time{}, false
}

func (s *scan) Dir(first bool, new, old []string, l log.Logger) (newDir []string, same []string, oldDir []string) {
	level.Debug(l).Log("old dir", old, "new dir", new)
	max := maxLen(new, old)
	newDir = make([]string, 0, max)
	same = make([]string, 0, max)
	oldDir = make([]string, 0, 10)
	if first {
		same = old
		return
	}
	if len(new) < 1 {
		oldDir = old
		return
	}
	for _, n := range new {
		newFlag := true
		for _, o := range old {
			if n == o {
				newFlag = false
				break
			}
		}
		if newFlag {
			newDir = append(newDir, n)
		} else {
			same = append(same, n)
		}
	}
	for _, o := range old {
		oldFlag := true
		for _, s := range same {
			if o == s {
				oldFlag = false
				break
			}
		}
		if oldFlag {
			oldDir = append(oldDir, o)
		}
	}

	return
}

func maxLen(new, old []string) int {
	if len(new) > len(old) {
		return len(new)
	}
	return len(old)
}
