package scan

import "sync"

var oldDir []string

type dirsCheck struct {
	l sync.Mutex
}

func NewDirs() *dirsCheck {
	return &dirsCheck{}
}
func (dc *dirsCheck) Get() []string {
	dc.l.Lock()
	defer dc.l.Unlock()
	return oldDir
}

func (dc *dirsCheck) Set(in []string) {
	dc.l.Lock()
	defer dc.l.Unlock()
	oldDir = in
}
