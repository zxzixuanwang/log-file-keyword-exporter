package conf

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var AppConfig *Config
var ConfigLogFile map[string]*List
var Ip string

type App struct {
	Name      string
	CpuNumber int
	Debug     bool
	Port      string
}

type Config struct {
	Log     Log      `json:"log,omitempty"`
	Info    Info     `json:"info,omitempty"`
	LogFile *LogFile `json:"logFile,omitempty"`
	Tsdb    Tsdb     `json:"tsdb,omitempty"`
	App     App      `json:"app,omitempty"`
}
type Log struct {
	Level        string `json:"level,omitempty"`
	FilePosition string `json:"filePosition,omitempty"`
}

type Info struct {
	Biz      string `json:"biz,omitempty"`
	Instance string `json:"instance,omitempty"`
}

type LogFile struct {
	Flush       int    `json:"flush,omitempty"`
	Save        int    `json:"save,omitempty"`
	List        []List `json:"list,omitempty"`
	Check       int    `json:"check,omitempty"`
	Ttl         int    `json:"ttl,omitempty"`
	PositionDir string `json:"positionDir,omitempty"`
}

type List struct {
	AppName        string   `json:"appName,omitempty"`
	KeyWords       []string `json:"keyWords,omitempty"`
	FilePosition   string   `json:"filePosition,omitempty"`
	Buff           int      `json:"buff,omitempty"`
	RulerName      string   `json:"rulerName,omitempty"`
	ResolveKeyWord []string `json:"resolveKeyWord,omitempty"`
}

type Tsdb struct {
	Address   string `json:"address,omitempty"`
	TimeOut   int    `json:"timeOut,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	RateGen   int    `json:"rateGen,omitempty"`
	Bucket    int    `json:"bucket,omitempty"`
}

func init() {
	var (
		cfgFile = pflag.StringP("config", "c", "", "config file")
	)

	pflag.Parse()

	if *cfgFile != "" {
		viper.SetConfigFile(*cfgFile)
	} else {
		viper.AddConfigPath("config")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic("read config error" + err.Error())
	}

	app := new(Config)
	if err := viper.Unmarshal(&app); err != nil {
		panic("load config error" + err.Error())
	}
	AppConfig = app
	if len(app.LogFile.List) > 0 {
		ConfigLogFile = make(map[string]*List, len(app.LogFile.List))
		for _, v := range app.LogFile.List {
			ConfigLogFile[v.AppName] = &List{
				AppName:      v.AppName,
				KeyWords:     v.KeyWords,
				FilePosition: v.FilePosition,
				Buff:         v.Buff,
				RulerName:    v.RulerName,
			}
		}
	}

	fmt.Println(viper.ConfigFileUsed())
	fmt.Println("config output", AppConfig)

}
