package tsdb

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/prometheus/prompb"
)

type PromRemoteIn struct {
	Address   string
	TimeOut   int
	UserAgent string
}

type PromRemote struct {
	l             log.Logger
	CounterName   string
	DefaultLabels []prompb.Label
	Address       string
	BasicUsername *string
	BasicPassword *string
	BearToken     *string
	// seconds
	ClietTimeOut int
	UA           string
	Help         string
	Url          *url.URL
	client       api.Client
}
type Request struct {
	Value     float64
	NewLabels []prompb.Label
}

type Options func(*PromRemote)

func WithUserAgent(UA string) Options {
	return func(pr *PromRemote) {
		pr.UA = UA
	}
}

func WithIpAddress(address string) Options {
	return func(in *PromRemote) {
		in.Address = address
	}
}

func WithLabels(req []prompb.Label) Options {
	return func(in *PromRemote) {
		in.DefaultLabels = req
	}
}

func WithLog(l log.Logger) Options {
	return func(pr *PromRemote) {
		pr.l = l
	}
}

func WithBasic(username, password string) Options {
	return func(in *PromRemote) {
		in.BasicUsername = &username
		in.BasicPassword = &password
	}
}

func WithTimeOut(timeOut int) Options {
	return func(pr *PromRemote) {
		pr.ClietTimeOut = timeOut
	}
}

func WithBearToken(token string) Options {
	return func(in *PromRemote) {
		in.BearToken = &token
	}
}

func NewProRemoteOpt() *PromRemote {
	return new(PromRemote)
}
func defaultProRemote() *PromRemote {

	return &PromRemote{
		Address:      "localhost:9090",
		ClietTimeOut: 60,
		UA:           "keyword-exporter",
		CounterName:  "keyword_appear_alert",
		Help:         "some keyword appear alert",
		l:            log.NewJSONLogger(os.Stdout),
	}
}

func NewProRemote(opt ...Options) (PromRemoteInterface, error) {
	pr := defaultProRemote()
	for _, v := range opt {
		v(pr)
	}

	url, err := url.Parse(pr.Address)
	if err != nil {
		level.Error(pr.l).Log("解析地址失败", err)
		return nil, err
	}
	pr.Url = url

	apiclient, err := api.NewClient(api.Config{
		Address: pr.Address,
		Client:  &http.Client{Timeout: time.Second * time.Duration(pr.ClietTimeOut)},
		//		RoundTripper: http.DefaultTransport,
	})
	if err != nil {
		level.Error(pr.l).Log("api client 创建失败", err)
		return nil, err
	}

	pr.client = apiclient
	return pr, nil
}

type PromRemoteInterface interface {
	Send(value float64, newLabels []prompb.Label)
}

func (pr *PromRemote) Send(value float64, newLabels []prompb.Label) {

	level.Debug(pr.l).Log("labels", newLabels)
	header := map[string]string{
		"User-Agent":                        pr.UA,
		"Content-Encoding":                  "snappy",
		"Content-Type":                      "application/x-protobuf",
		"X-Prometheus-Remote-Write-Version": "0.1.0",
	}

	newLabels = append(newLabels, prompb.Label{
		Name:  LABEL_NAME,
		Value: pr.CounterName,
	})
	// Create a new Prometheus write request.
	writeRequest := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{{
			Labels: newLabels,
			Samples: []prompb.Sample{
				{
					Value:     value,
					Timestamp: time.Now().UnixMilli(),
				},
			}}},

		Metadata: []prompb.MetricMetadata{{
			Type:             prompb.MetricMetadata_HISTOGRAM,
			MetricFamilyName: pr.CounterName,
			Help:             pr.Help,
		}},
	}

	data, err := writeRequest.Marshal()
	if err != nil {
		level.Error(pr.l).Log("marsh proto data err", err)
		return
	}

	ctx := context.Background()

	level.Debug(pr.l).Log("timeout", pr.ClietTimeOut)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(pr.ClietTimeOut)*time.Second*2)
	defer cancel()

	data = snappy.Encode(nil, data)
	err = pr.post(data, ctx, header)
	if err != nil {
		level.Warn(pr.l).Log("发送请求失败", err, "进入重试", "10 second")
		nt := time.NewTicker(time.Second * 10)

		for {
			select {
			case <-nt.C:
				level.Debug(pr.l).Log("重试发送", "10 seconds", "list", newLabels)

				err = pr.post(data, ctx, header)
				if err == nil {
					level.Info(pr.l).Log("重试发送", "成功", "list", newLabels)
					return
				}
			case <-ctx.Done():
				level.Error(pr.l).Log("发送请求错误", "超时", "list", newLabels)
				return
			}
		}
	} else {
		level.Info(pr.l).Log("发送tsdb", "成功", "list", newLabels)
	}
}

func (pr *PromRemote) post(req []byte, ctx context.Context, headers ...map[string]string) error {
	httpReq, err := http.NewRequest("POST", pr.Address, bytes.NewReader(req))
	if err != nil {
		level.Warn(pr.l).Log("create remote write request got error", err)
		return err
	}

	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("User-Agent", pr.UA)
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	if len(headers) > 0 {
		for k, v := range headers[0] {
			httpReq.Header.Set(k, v)
		}
	}

	if pr.BasicUsername != nil && pr.BasicPassword != nil {
		httpReq.SetBasicAuth(*pr.BasicUsername, *pr.BasicPassword)
	}

	resp, body, err := pr.client.Do(ctx, httpReq)
	if err != nil {
		level.Warn(pr.l).Log("push data with remote write request got error", err, "response body", string(body))
		return err
	}

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("push data with remote write request got status code: %v, response body: %s", resp.StatusCode, string(body))
		level.Error(pr.l).Log("err", err)
		return err
	}

	return nil
}
