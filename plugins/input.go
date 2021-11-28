package plugins

type Initializer interface {
	Init() error
}

type Input interface {
	Series() Series
	Gather() ([]interface{}, error)
}

type Series struct {
	Series   []string
	Selected []string
}

var Inputs = make(map[string]func() Input)

func Add(name string, inputFunc func() Input) {
	Inputs[name] = inputFunc
}
