package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/bingoohuang/berf/pkg/blow"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/sigx"
	_ "github.com/joho/godotenv/autoload"
)

var (
	pVersion = fla9.Bool("version", false, "Show version and exit")
	pInit    = fla9.Bool("init", false, "Create initial ctl and exit")
	pProfile = fla9.String("pprof", "", "write CPU profile to file, e.g. cpu.prof")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
	sigx.RegisterSignalProfile()
}

func main() {
	if *pProfile != "" {
		f, err := os.Create(*pProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	blow.StartBlow()
}
