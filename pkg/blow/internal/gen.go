package internal

import (
	"regexp"
	"sync"

	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/bingoohuang/jj"
)

var Valuer jj.Substitute = &valuer{Map: make(map[string]interface{})}

type valuer struct {
	Map map[string]interface{}
	sync.RWMutex
}

func (v *valuer) Register(fn string, f jj.SubstitutionFn) {
	jj.DefaultSubstituteFns.Register(fn, f)
}

var cacheSuffix = regexp.MustCompile(`^(.+)_\d+`)

func (v *valuer) Value(name, params string) interface{} {
	pureName := name
	subs := cacheSuffix.FindStringSubmatch(name)
	if len(subs) > 0 {
		pureName = subs[1]
		v.RLock()
		x, ok := v.Map[name]
		v.RUnlock()
		if ok {
			return x
		}
	}

	x := jj.DefaultGen.Value(pureName, params)

	if len(subs) > 0 {
		v.Lock()
		v.Map[name] = x
		v.Unlock()
	}
	return x
}

type StringMode int

const (
	MayJSON StringMode = iota
	SureJSON
)

func Gen(s string, mode StringMode) string {
	gen := jj.NewGenContext(Valuer)
	if mode == SureJSON || mode == MayJSON && jj.Valid(s) {
		return gen.Gen(s)
	}

	return vars.ToString(vars.ParseExpr(s).Eval(Valuer))
}
