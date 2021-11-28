package perf

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/multierr"

	"github.com/bingoohuang/gg/pkg/ss"
)

type JSONLogFile struct {
	F *os.File
	*sync.Mutex
	Dry     bool
	Closed  bool
	HasRows bool
}

const DrySuffix = ":dry"

func IsFileNameDry(file string) bool {
	return strings.HasSuffix(file, DrySuffix)
}

func NewFile(file string) *JSONLogFile {
	dry := IsFileNameDry(file)
	if file == "" {
		file = "perf_" + time.Now().Format(`200601021504`) + ".log"
	} else if dry {
		file = strings.TrimSuffix(file, DrySuffix)
	}
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0o600)
	logFile := &JSONLogFile{F: f, Mutex: &sync.Mutex{}, Dry: dry}
	if err != nil {
		log.Printf("E! Fail to open log file %s error: %v", file, err)
	}
	if n, err := f.Seek(0, io.SeekEnd); err != nil {
		log.Printf("E! fail to seek file %s error: %v", file, err)
	} else if n == 0 {
		f.WriteString("[]\n")
	} else {
		logFile.HasRows = true
	}
	return logFile
}

func (f JSONLogFile) ReadAll() []byte {
	if f.F == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()

	if f.Closed {
		data, _ := os.ReadFile(f.F.Name())
		return data
	}

	f.F.Seek(0, io.SeekStart)
	defer f.F.Seek(0, io.SeekEnd)

	data, err := ReadFile(f.F)
	if err != nil {
		log.Printf("E! fail to read log file %s, error: %v", f.F.Name(), err)
	}
	return data
}

// ReadFile reads the named file and returns the contents.
// A successful call returns err == nil, not err == EOF.
// Because ReadFile reads the whole file, it does not treat an EOF from Read
// as an error to be reported.
func ReadFile(f *os.File) ([]byte, error) {
	var size int
	if info, err := f.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	size++ // one byte for final read at EOF

	// If a file claims a small size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}

func (f *JSONLogFile) WriteJSON(data []byte) error {
	if f.F == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()

	f.F.Seek(-2, io.SeekEnd) // \n]
	var err0 error

	if !f.HasRows {
		f.HasRows = true
		_, err0 = f.F.WriteString("\n")
	} else {
		_, err0 = f.F.WriteString(",\n")
	}
	_, err1 := f.F.Write(data)
	_, err2 := f.F.WriteString("\n]")
	return multierr.Combine(err0, err1, err2)
}

func (f JSONLogFile) IsDry() bool { return f.Dry }

func (f *JSONLogFile) Close() error {
	if f.F == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()
	f.Closed = true
	return f.F.Close()
}

func NewFeatureMap(features string) FeatureMap {
	m := make(FeatureMap)
	m.Setup(features)
	return m
}

// FeatureMap defines a feature map.
type FeatureMap map[string]bool

// Setup sets up a feature map by features string, which separates feature names by comma.
func (f *FeatureMap) Setup(features string) {
	for _, feature := range strings.Split(strings.ToLower(features), ",") {
		if v := strings.TrimSpace(feature); v != "" {
			(*f)[v] = true
		}
	}
}

// HasFeature tells the feature map contains a features.
func (f *FeatureMap) HasFeature(feature string) bool {
	return (*f)[feature] || (*f)[strings.ToLower(feature)]
}

// HasAny tells the feature map contains any of the features.
func (f *FeatureMap) HasAny(features ...string) bool {
	for _, feature := range features {
		if f.HasFeature(feature) {
			return true
		}
	}

	return false
}

type PushResult int

const (
	PushOK PushResult = iota
	PushOKDrop
	PushFail
)

func TryWrite(c chan []byte, v []byte) PushResult {
	select {
	case c <- v:
		return PushOK
	default:
	}

	dropped := false
	select {
	case <-c:
		dropped = true
	default:
	}

	select {
	case c <- v:
		if dropped {
			return PushOKDrop
		} else {
			return PushOK
		}
	default:
		return PushFail
	}
}

type WidthHeight struct {
	W, H int
}

func (h WidthHeight) WidthPx() string  { return fmt.Sprintf("%dpx", h.W) }
func (h WidthHeight) HeightPx() string { return fmt.Sprintf("%dpx", h.H) }

func ParseWidthHeight(val string, defaultWidth, defaultHeight int) WidthHeight {
	wh := WidthHeight{
		W: defaultWidth,
		H: defaultHeight,
	}
	if val != "" {
		val = strings.ToLower(val)
		parts := strings.SplitN(val, "x", 2)
		if len(parts) == 2 {
			if v := ss.ParseInt(parts[0]); v > 0 {
				wh.W = v
			}
			if v := ss.ParseInt(parts[1]); v > 0 {
				wh.H = v
			}
		}
	}
	return wh
}

type GoroutineIncr struct {
	Up   int
	Dur  time.Duration
	Down int
}

func (i GoroutineIncr) Modifier() string {
	return ss.If(i.Up > 0, "max ", "")
}

func (i GoroutineIncr) IsEmpty() bool {
	return i.Up <= 0 && i.Down <= 0
}

// ParseGoIncr parse a GoIncr expressions like:
// 1. (empty) => GoroutineIncr{}
// 2. 0       => GoroutineIncr{}
// 3. 1       => GoroutineIncr{Up: 1}
// 4. 1:10s   => GoroutineIncr{Up: 1, Dur:10s}
// 5. 1:10s:1 => GoroutineIncr{Up: 1, Dur:10s, Down:1}
func ParseGoIncr(s string) GoroutineIncr {
	s = strings.TrimSpace(s)
	if s == "" {
		return GoroutineIncr{Up: 0, Dur: 0}
	}

	var err error
	parts := ss.Split(s, ss.WithIgnoreEmpty(true), ss.WithTrimSpace(true), ss.WithSeps(":"))
	v := ss.ParseInt(parts[0])
	ret := GoroutineIncr{Up: v, Dur: 0}
	if len(parts) >= 2 {
		ret.Dur, err = time.ParseDuration(parts[1])
		if err != nil {
			log.Printf("W! %s is invalid", s)
		}
	}
	if len(parts) >= 3 {
		ret.Down = ss.ParseInt(parts[2])
	}

	return ret
}

// OpenBrowser go/src/cmd/internal/browser/browser.go
func OpenBrowser(url string) bool {
	var cmds [][]string
	if exe := os.Getenv("BROWSER"); exe != "" {
		cmds = append(cmds, []string{exe})
	}
	switch runtime.GOOS {
	case "darwin":
		cmds = append(cmds, []string{"/usr/bin/open"})
	case "windows":
		cmds = append(cmds, []string{"cmd", "/c", "start"})
	default:
		if os.Getenv("DISPLAY") != "" {
			// xdg-open is only for use in a desktop environment.
			cmds = append(cmds, []string{"xdg-open"})
		}
	}
	cmds = append(cmds, []string{"chrome"}, []string{"google-chrome"}, []string{"chromium"}, []string{"firefox"})
	for _, args := range cmds {
		cmd := exec.Command(args[0], append(args[1:], url)...)
		if cmd.Start() == nil && WaitTimeout(cmd, 3*time.Second) {
			return true
		}
	}
	return false
}

// WaitTimeout reports whether the command appears to have run successfully.
// If the command runs longer than the timeout, it's deemed successful.
// If the command runs within the timeout, it's deemed successful if it exited cleanly.
func WaitTimeout(cmd *exec.Cmd, timeout time.Duration) bool {
	ch := make(chan error, 1)
	go func() {
		ch <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		return true
	case err := <-ch:
		return err == nil
	}
}

func SleepContext(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	return
}

func GetFreePortStart(port int) int {
	for i := 0; i < 100; i++ {
		if IsPortFree(port) {
			return port
		}
		port++
	}

	return 0
}

// IsPortFree tells whether the port is free or not
func IsPortFree(port int) bool {
	l, err := ListenPort(port)
	if err != nil {
		return false
	}

	_ = l.Close()
	return true
}

// ListenPort listens on port
func ListenPort(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf(":%d", port))
}

func ExitIfErr(err error) {
	if err != nil {
		Exit(err.Error())
	}
}

func Exit(msg string) {
	fmt.Fprintln(os.Stderr, "blow: "+msg)
	os.Exit(1)
}

func MergeCodes(codes []string) string {
	n := 0
	last := ""
	merged := ""
	for _, code := range codes {
		if code != last {
			if last != "" {
				merged = mergeCodes(merged, n, last)
			}
			last = code
			n = 1
		} else {
			n++
		}
	}

	if n > 0 {
		merged = mergeCodes(merged, n, last)
	}

	return merged
}

func mergeCodes(merged string, n int, last string) string {
	if merged != "" {
		merged += ","
	}
	if n > 1 {
		merged += fmt.Sprintf("%sx%d", last, n)
	} else {
		merged += fmt.Sprintf("%s", last)
	}
	return merged
}

// ParseFileArg parse an argument which represents a string content,
// or @file to represents the file's content.
func ParseFileArg(arg string) []byte {
	if strings.HasPrefix(arg, "@") {
		f := (arg)[1:]
		if v, err := ioutil.ReadFile(f); err != nil {
			log.Fatalf("failed to read file %s, error: %v", f, err)
			return nil
		} else {
			return v
		}
	}

	return []byte(arg)
}
