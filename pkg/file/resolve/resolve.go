package resolve

type Status int

const (
	StatusFiring Status = iota + 1
	StatusResolved
)

type Option func(*Resolved)

type Resolved struct {
	AppName    []string
	statusSave map[string]Status
}

type ResolveInterface interface {
	Alarm(appName string)
	Resolve(appName string)
	Range(f func(appName string))
}

func NewResolver(opt ...Option) ResolveInterface {
	r := defaultResolve()
	for _, v := range opt {
		v(r)
	}
	return r
}

func WithAppName(appName []string) Option {
	return func(r *Resolved) {
		r.AppName = appName
	}
}

func defaultResolve() *Resolved {
	return &Resolved{}
}

func (r *Resolved) Alarm(appName string) {
	r.statusSave[appName] = StatusFiring
}

func (r *Resolved) Resolve(appName string) {
	r.statusSave[appName] = StatusResolved
}

func (r *Resolved) newStatus() {
	r.statusSave = make(map[string]Status, len(r.AppName))
	for _, v := range r.AppName {
		r.statusSave[v] = StatusResolved
	}
}

func (r *Resolved) Range(f func(appName string)) {
	for k, v := range r.statusSave {
		if v == StatusFiring {
			f(k)
		}
	}
}
