package savepostion

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/zxzixuanwang/log-file-keyword-exporter/conf"
)

var (
	appInfo = make(map[string]*FileInfo, 2*len(conf.AppConfig.LogFile.List))
	lock    = sync.Mutex{}
)

type SavePos struct {
	FilePosition string
	l            log.Logger
}

type FIInput struct {
	FileName string `json:"fileName,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	AppName  string `json:"appName,omitempty"`
}

func set(in *FileInfo) {
	lock.Lock()
	defer lock.Unlock()

	appInfo[in.AppName] = &FileInfo{
		Offset:   in.Offset,
		AppName:  in.AppName,
		FileName: in.FileName,
	}
}

func Get(key string) *FileInfo {
	return appInfo[key]
}

func NewSavePos(filePosition string, l log.Logger) *SavePos {
	return &SavePos{
		FilePosition: filePosition,
		l:            l,
	}
}
func (sp *SavePos) HotSave(v *FIInput) {
	set(&FileInfo{
		Offset:   v.Offset,
		AppName:  v.AppName,
		FileName: v.FileName,
	})
}

func (sp *SavePos) PatchSave(in []*FIInput) (map[string]*FileInfo, error) {
	for _, v := range in {

		set(&FileInfo{
			Offset:   v.Offset,
			AppName:  v.AppName,
			FileName: v.FileName,
		})
	}

	content, err := json.Marshal(appInfo)
	if err != nil {
		level.Error(sp.l).Log("marshal 失败", err)
		return appInfo, err
	}

	level.Info(sp.l).Log("save file content", string(content))
	err = sp.SaveFile(content)
	if err != nil {
		level.Error(sp.l).Log("write content in file failed, err", err)
		return appInfo, err
	}
	return appInfo, nil
}

func (sp *SavePos) SaveFile(content []byte) error {
	f, err := os.OpenFile(sp.FilePosition, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		level.Error(sp.l).Log("save file error", err)
		return err
	}
	// 关闭文件
	defer f.Close()
	_, err = f.Write(content)
	return err
}

func (sp *SavePos) Load(first bool) map[string]*FileInfo {
	level.Debug(sp.l).Log("load file first", first)
	content, err := sp.LoadFile(first)
	if err != nil {
		level.Error(sp.l).Log("load file error", err)
		return appInfo
	}
	level.Info(sp.l).Log("load file content", string(content))
	var tempInfo map[string]FileInfo
	err = json.Unmarshal(content, &tempInfo)
	if err != nil {
		level.Error(sp.l).Log("Unmarshal file error", err)
		return appInfo
	}
	for _, v := range tempInfo {
		set(&v)
	}

	return appInfo
}

func (sp *SavePos) LoadFile(first bool) ([]byte, error) {
	var (
		f   *os.File
		err error
	)
	f, err = os.OpenFile(sp.FilePosition, os.O_RDONLY, 0666)

	if errors.Is(err, os.ErrNotExist) {
		f, err = os.Create(sp.FilePosition)
	}
	if err != nil {
		level.Error(sp.l).Log("load file error", err)
		if first {
			panic(err)
		}
		return nil, err
	}
	defer f.Close()

	nr := bufio.NewReader(f)
	var chunk []byte
	buf := make([]byte, 1024)
	for {
		n, err := nr.Read(buf)
		if err != nil {
			if n == 0 {
				break
			}
			if errors.Is(err, io.EOF) {
				break
			} else {
				level.Error(sp.l).Log("read line error", err)
				return nil, err
			}
		}
		chunk = append(chunk, buf[:n]...)

	}
	return chunk, nil
}

type FileInfo struct {
	Offset   int64  `json:"offset,omitempty"`
	AppName  string `json:"appName,omitempty"`
	FileName string `json:"fileName,omitempty"`
}
