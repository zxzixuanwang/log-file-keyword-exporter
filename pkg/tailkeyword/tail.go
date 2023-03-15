package tailkeyword

import (
	"context"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hpcloud/tail"
	"github.com/zxzixuanwang/log-file-keyword-exporter/conf"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/check"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/file/resolve"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/limit"
	"github.com/zxzixuanwang/log-file-keyword-exporter/pkg/tsdb"
)

type TailWordInfo struct {
	L       log.Logger
	Minute  int
	Buff    int
	Pro     tsdb.PromRemoteInterface
	Limit   limit.LimitInterface
	Resolve resolve.ResolveInterface
}

func NewTailWordInfo(in *TailWordInfo) TailWordInfoInterface {
	return in
}

type TailWordInfoInterface interface {
	TailWord(in *TailWordIn, ctx context.Context, filter func(msg string, keywords []string) *string)
}

type TailWordIn struct {
	FileName     string
	ReOpen       bool
	Follow       bool
	Offset       int64
	Whence       int
	MustExist    bool
	Poll         bool
	KeyWord      []string
	ResolvedWord []string
	RulerName    string
	AppName      string
	Ctx          context.Context
}

func (twi *TailWordInfo) TailWord(in *TailWordIn, ctx context.Context, filter func(msg string, keyword []string) *string) {
	config := tail.Config{
		Location:  &tail.SeekInfo{Offset: in.Offset, Whence: in.Whence},
		ReOpen:    in.ReOpen,
		MustExist: in.MustExist,
		Poll:      in.Poll,
		Pipe:      false,
		Follow:    in.Follow,
	}
	tails, err := tail.TailFile(in.FileName, config)
	if err != nil {
		level.Error(twi.L).Log("tail file failed, err", err)
		return
	}

	var (
		line *tail.Line
		ok   bool
	)
	check.Insert(in.FileName, tails)
	resoFlag := (len(in.ResolvedWord) > 0)
	//var builder strings.Builder
	/* 	t := time.NewTicker(time.Minute * time.Duration(twi.Minute)) */

	for {
		select {
		case line, ok = <-tails.Lines: //遍历chan，读取日志内容
			if !ok {
				level.Error(twi.L).Log("tail file close reopen, filename:", tails.Filename)
				continue
			}
			level.Debug(twi.L).Log("tail content", line.Text)
			if findKeyWord := filter(line.Text, in.KeyWord); findKeyWord != nil {
				twi.Limit.LimitSend(in.FileName, func() {
					go twi.Pro.Send(float64(1),
						tsdb.NewPromLabels(in.AppName, in.FileName, conf.Ip,
							tsdb.WithOthers(map[string][]string{"keywords": {*findKeyWord},
								"rulerName": {in.RulerName}}),
						).
							GenLabels(),
					)
					if resoFlag {
						twi.Resolve.Alarm(in.AppName)
					}
				})
			}
			if resoFlag && filter(line.Text, in.ResolvedWord) != nil {
				twi.Resolve.Resolve(in.AppName)
			}

		case <-ctx.Done():
			if err = tails.Stop(); err != nil {
				level.Error(twi.L).Log("dying name", tails.Filename, "err", err)
				return
			}
			level.Debug(twi.L).Log("dying name", tails.Filename)
			return

		}
	}
}
