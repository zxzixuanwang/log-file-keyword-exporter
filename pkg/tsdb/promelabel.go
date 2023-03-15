package tsdb

import (
	"github.com/prometheus/prometheus/prompb"
)

type PromLabels struct {
	AppName     string
	Pattern     string
	IPinfo      string
	Other       map[string][]string
	MetricsName string
}

type PromOptions func(*PromLabels)

const LABEL_NAME = "__name__"

func NewPromLabels(appName, pattern string, ipinfo string, opt ...PromOptions) *PromLabels {
	newPl := &PromLabels{
		AppName: appName,
		Pattern: pattern,
		IPinfo:  ipinfo,
	}
	for _, o := range opt {
		o(newPl)
	}

	return newPl
}

func WithOthers(other map[string][]string) PromOptions {
	return func(pli *PromLabels) {
		pli.Other = other
	}
}

func (pl *PromLabels) GenLabels() []prompb.Label {
	tempLabels := []prompb.Label{
		{
			Name:  "app_name",
			Value: pl.AppName,
		},
		{
			Name:  "log_position",
			Value: pl.Pattern,
		},
		{
			Name:  "instance",
			Value: pl.IPinfo,
		},
	}

	if len(pl.Other) > 0 {
		res := make([]prompb.Label, 0, len(pl.Other)+2)
		for k, v := range pl.Other {
			res = append(res, prompb.Label{
				Name:  k,
				Value: appendStr(v),
			})
		}
		return append(res, tempLabels...)
	}
	return tempLabels
}

func appendStr(in []string) string {
	appendStr := ""
	for i := 0; i < len(in)-1; i++ {
		appendStr += (in[i] + ",")
	}

	return appendStr + in[len(in)-1]
}
