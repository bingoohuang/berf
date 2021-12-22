module github.com/bingoohuang/berf

go 1.17

require (
	github.com/AdhityaRamadhanus/fasthttpcors v0.0.0-20170121111917-d4c07198763a
	github.com/axiomhq/hyperloglog v0.0.0-20211021164851-7f2dfa314bc7
	github.com/beorn7/perks v1.0.1
	github.com/bingoohuang/gg v0.0.0-20211222045333-8c457dbf2025
	github.com/bingoohuang/jj v0.0.0-20211125042349-4752d135093f
	github.com/dustin/go-humanize v1.0.0
	github.com/go-echarts/go-echarts/v2 v2.2.4
	github.com/gobwas/glob v0.2.3
	github.com/karrick/godirwalk v1.16.1
	github.com/mattn/go-isatty v0.0.15-0.20210929170527-d423e9c6c3bf
	github.com/mattn/go-runewidth v0.0.13
	github.com/mitchellh/go-homedir v1.1.0
	github.com/shirou/gopsutil/v3 v3.21.10
	github.com/thoas/go-funk v0.9.2-0.20211112205042-658cf4758bf8
	github.com/valyala/fasthttp v1.31.0
	go.uber.org/multierr v1.6.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/sys v0.0.0-20211013075003-97ac67df715c
)

require (
	github.com/Pallinder/go-randomdata v1.2.0 // indirect
	github.com/andybalholm/brotli v1.0.2 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/pbnjay/pixfont v0.0.0-20200714042608-33b744692567 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20210510120150-4163338589ed // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

//replace github.com/bingoohuang/gg => /Users/bingoobjca/github/gg

replace github.com/shirou/gopsutil/v3 => ../gopsutil
