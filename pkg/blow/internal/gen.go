package internal

import (
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/bingoohuang/jj"
)

var Valuer = jj.NewCachingSubstituter()

type StringMode int

const (
	IgnoreJSON StringMode = iota
	MayJSON
	SureJSON
)

var gen = jj.NewGenContext(Valuer)

func Gen(s string, mode StringMode) string {
	if mode == SureJSON || mode == MayJSON && jj.Valid(s) {
		gs, _ := gen.Gen(s)
		return gs
	}

	eval, _ := vars.ParseExpr(s).Eval(Valuer)
	return vars.ToString(eval)
}
