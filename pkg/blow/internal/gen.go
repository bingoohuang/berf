package internal

import (
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/bingoohuang/jj"
)

type Valuer struct {
	Map map[Keep]interface{}
}

func NewValuer() *Valuer {
	return &Valuer{Map: make(map[Keep]interface{})}
}

func (v *Valuer) Register(fn string, f jj.SubstitutionFn) {
	jj.DefaultSubstituteFns.Register(fn, f)
}

type Keep struct {
	Keep string
	Name string
}

func (v *Valuer) Value(name, params string) interface{} {
	keep := Keep{}
	jj.ParseConf(params, &keep)

	if keep.Keep == "" {
		return jj.DefaultGen.Value(name, params)
	}

	keep.Name = name

	if x, ok := v.Map[keep]; ok {
		return x
	}

	x := jj.DefaultGen.Value(name, params)
	v.Map[keep] = x
	return x
}

type StringMode int

const (
	MayJSON StringMode = iota
	NotJSON
	SureJSON
)

func Gen(s string, mode StringMode) string {
	valuer := NewValuer()
	gen := jj.NewGenContext(valuer)
	if mode == SureJSON || mode == MayJSON && jj.Valid(s) {
		return gen.Gen(s)
	}

	return vars.ToString(vars.ParseExpr(s).Eval(valuer))
}
